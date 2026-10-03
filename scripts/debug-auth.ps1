param(
    [string]$BaseUrl = 'http://127.0.0.1:8080',
    [string]$RedisContainer = 'note-redis-learning',
    [string]$Account,
    [string]$Password = 'Debug_12345',
    [switch]$PauseEachStep,
    [switch]$CreateTodo
)

$ErrorActionPreference = 'Stop'
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()

function Send-AuthRequest([string]$Path, [object]$Body) {
    $options = @{
        Uri = "$BaseUrl/api$Path"
        Method = 'Post'
        WebSession = $session
        Headers = @{ 'X-Note-Request' = '1' }
        ContentType = 'application/json'
        SkipHttpErrorCheck = $true
    }
    if ($null -ne $Body) { $options.Body = $Body | ConvertTo-Json -Compress }
    $response = Invoke-WebRequest @options
    $parsed = if ($response.Content) { $response.Content | ConvertFrom-Json } else { $null }
    [pscustomobject]@{ Status = [int]$response.StatusCode; Body = $parsed }
}

function Read-RefreshCookie {
    $cookie = $session.Cookies.GetCookies([uri]$BaseUrl)['note_refresh']
    if ($null -eq $cookie) { return '' }
    return $cookie.Value
}

function Get-RefreshKey([string]$Raw) {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Raw)
    $digest = [System.Security.Cryptography.SHA256]::HashData($bytes)
    'note:refresh:' + [Convert]::ToHexString($digest).ToLowerInvariant()
}

function Show-RedisSession([string]$Label, [string]$Key) {
    $value = & docker exec $RedisContainer redis-cli --raw GET $Key
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Redis. Check RedisContainer.' }
    $ttl = & docker exec $RedisContainer redis-cli --raw TTL $Key
    Write-Host "$Label key=$Key user_id=$value ttl_seconds=$ttl"
}

function Show-JwtClaims([string]$Token) {
    $payload = $Token.Split('.')[1].Replace('-', '+').Replace('_', '/')
    $payload = $payload.PadRight($payload.Length + ((4 - $payload.Length % 4) % 4), '=')
    $claims = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($payload)) | ConvertFrom-Json
    [pscustomobject]@{
        user_id = $claims.user_id
        issuer = $claims.iss
        issued_at = [DateTimeOffset]::FromUnixTimeSeconds($claims.iat).ToOffset([TimeSpan]::FromHours(8)).ToString('o')
        expires_at = [DateTimeOffset]::FromUnixTimeSeconds($claims.exp).ToOffset([TimeSpan]::FromHours(8)).ToString('o')
        lifetime_seconds = $claims.exp - $claims.iat
    } | Format-List | Out-Host
    # Decoding shows the payload; server-side TokenManager.Parse validates the signature.
}

function Step([string]$Title) {
    Write-Host "`n$Title" -ForegroundColor Cyan
    if ($PauseEachStep) { [void](Read-Host 'Press Enter to send the next request') }
}

if (-not $Account) {
    $tag = [guid]::NewGuid().ToString('N').Substring(0, 10)
    $Account = "debug_$tag@example.com"
    $registered = Send-AuthRequest '/auth/register' @{ name = "debug_$tag"; email = $Account; password = $Password }
    if ($registered.Status -ne 201) { throw "Register failed: $($registered.Status) $($registered.Body.error)" }
    Write-Host "Created local debug account: $Account"
}

try {
    Step '1. Login: JWT in JSON, refresh token in Cookie, hashed session in Redis'
    $login = Send-AuthRequest '/auth/login' @{ account = $Account; password = $Password }
    if ($login.Status -ne 200) { throw "Login failed: $($login.Status) $($login.Body.error)" }
    $access = $login.Body.access_token
    $oldRaw = Read-RefreshCookie
    if (-not $oldRaw) { throw 'No refresh Cookie: check NOTE_COOKIE_SECURE for local HTTP.' }
    $oldKey = Get-RefreshKey $oldRaw
    Write-Host "HTTP=$($login.Status) user_id=$($login.Body.id) refresh_cookie_length=$($oldRaw.Length)"
    Show-JwtClaims $access
    Show-RedisSession 'After login' $oldKey

    Step '2. Protected request: JWT verifies identity; ordinary API does not rotate refresh session'
    $headers = @{ Authorization = "Bearer $access" }
    $groups = Invoke-WebRequest "$BaseUrl/api/groups" -Headers $headers -SkipHttpErrorCheck
    Write-Host "GET /api/groups HTTP=$([int]$groups.StatusCode)"
    Show-RedisSession 'After ordinary API' $oldKey

    if ($CreateTodo) {
        Step 'Optional. Create Group and Todo with the logged-in user'
        $group = Invoke-RestMethod "$BaseUrl/api/groups" -Method Post -Headers $headers -ContentType 'application/json' -Body (@{ name = 'JWT debugging group' } | ConvertTo-Json)
        $todoBody = @{ group_id = $group.id; title = 'JWT debugging Todo'; starts_at = '2026-10-03T09:00:00+08:00'; repeat_mode = 'once' } | ConvertTo-Json
        $todo = Invoke-RestMethod "$BaseUrl/api/todos" -Method Post -Headers $headers -ContentType 'application/json' -Body $todoBody
        Write-Host "Created group_id=$($group.id) todo_id=$($todo.id) creator_id=$($todo.creator_id)"
    }

    Step '3. Refresh: old Redis key disappears, new key and Cookie are issued'
    # JWT timestamps have second precision; ensure this demonstration crosses a second.
    Start-Sleep -Seconds 1
    $refresh = Send-AuthRequest '/auth/refresh' $null
    if ($refresh.Status -ne 200) { throw "Refresh failed: $($refresh.Status) $($refresh.Body.error)" }
    $newRaw = Read-RefreshCookie
    $newKey = Get-RefreshKey $newRaw
    Write-Host "HTTP=$($refresh.Status) access_changed=$($access -ne $refresh.Body.access_token) refresh_changed=$($oldRaw -ne $newRaw)"
    Show-JwtClaims $refresh.Body.access_token
    Show-RedisSession 'Old session' $oldKey
    Show-RedisSession 'New session' $newKey

    Step '4. Logout: current refresh session is revoked'
    $logout = Send-AuthRequest '/auth/logout' $null
    if ($logout.Status -ne 204) { throw "Logout failed: $($logout.Status) $($logout.Body.error)" }
    Write-Host "HTTP=$($logout.Status) cookie_cleared=$(-not (Read-RefreshCookie))"
    Show-RedisSession 'After logout' $newKey

    Step '5. Verify the boundary: refresh returns 401; already-issued access JWT still works until expiry'
    $afterLogout = Send-AuthRequest '/auth/refresh' $null
    $oldAccess = Invoke-WebRequest "$BaseUrl/api/groups" -Headers $headers -SkipHttpErrorCheck
    Write-Host "Refresh HTTP=$($afterLogout.Status); original JWT GET /api/groups HTTP=$([int]$oldAccess.StatusCode)"
}
finally {
    if (Read-RefreshCookie) {
        try { $null = Send-AuthRequest '/auth/logout' $null } catch { Write-Warning 'Debug refresh session could not be revoked.' }
    }
}
