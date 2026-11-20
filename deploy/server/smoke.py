"""Two-account deployment check. Creates clearly labelled test accounts/group."""
import argparse
import json
import secrets
import urllib.request
import urllib.error
import urllib.parse
import struct
import zlib

parser = argparse.ArgumentParser()
parser.add_argument('base', help='API root, e.g. https://files.gis.epawse.xyz/p/note/api')
args = parser.parse_args()
base = args.base.rstrip('/')

def request(path, data=None, token=None):
    headers = {'Content-Type': 'application/json', 'X-Note-Request': '1'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(base + path, data=json.dumps(data).encode() if data is not None else None, headers=headers)
    with urllib.request.urlopen(req, timeout=20) as res:
        return json.load(res) if res.status != 204 else None

def account(name):
    email = 'smoke-' + secrets.token_hex(6) + '@example.invalid'
    password = secrets.token_hex(8)
    request('/auth/register', {'name': name, 'email': email, 'password': password})
    return request('/auth/login', {'account': email, 'password': password})['access_token']

def stream(token):
    req = urllib.request.Request(base + '/notifications/stream', headers={'Authorization':'Bearer '+token, 'Accept':'text/event-stream'})
    return urllib.request.urlopen(req, timeout=20)

def event(stream):
    name = ''
    for _ in range(100):
        line = stream.readline().decode().strip()
        if line.startswith('event:'): name = line[6:].strip()
        if line.startswith('data:'): return name, json.loads(line[5:])
    raise AssertionError('No SSE data')

owner, user = account('部署验收群主'), account('部署验收申请人')
g = request('/groups', {'name':'部署验收群', 'policy':'approval'}, owner)
with stream(owner) as a, stream(user) as b:
    assert event(a)[0] == 'notification.snapshot'
    assert event(b)[0] == 'notification.snapshot'
    req = request('/groups/join', {'code':g['code']}, user)
    name, pending = event(a)
    assert name in ('notification.change', 'notification.snapshot') and pending['unread_count'] == 1, (name, pending)
    original = pending.get('upserts', pending.get('items'))[0]
    assert original['request_status'] == 'pending'
    request('/groups/join-requests/' + str(req['id']) + '/review', {'decision':'accept'}, owner)
    name, reviewed = event(a)
    assert name in ('notification.change', 'notification.snapshot') and reviewed['unread_count'] == 0
    updated = reviewed.get('upserts', reviewed.get('items'))[0]
    assert updated['id'] == original['id']
    assert updated['request_status'] == 'accepted'
    name, result = event(b)
    assert name in ('notification.change', 'notification.snapshot') and result['unread_count'] == 1
    received = result.get('upserts', result.get('items'))[0]
    assert received['type'] == 'join_reviewed'
    assert received['id'] != original['id']
    assert any(item['id'] == g['id'] for item in request('/groups', token=user))
print('PASS: registration, login, application, review, both SSE streams, unread counts and group membership')


def png():
    def chunk(kind, data):
        return struct.pack("!I",len(data))+kind+data+struct.pack("!I",zlib.crc32(kind+data)&0xffffffff)
    return b"\x89PNG\r\n\x1a\n"+chunk(b"IHDR",struct.pack("!2I5B",2,2,8,2,0,0,0))+chunk(b"IDAT",zlib.compress((b"\0"+b"\xf3\xb5\x1b"*2)*2))+chunk(b"IEND",b"")


def upload(path, token):
    boundary = "note" + secrets.token_hex(12)
    body = ("--"+boundary+'\r\nContent-Disposition: form-data; name="avatar"; filename="test.png"\r\nContent-Type: image/png\r\n\r\n').encode()+png()+ ("\r\n--"+boundary+"--\r\n").encode()
    req = urllib.request.Request(base+path,data=body,method="PUT",headers={"Authorization":"Bearer "+token,"Content-Type":"multipart/form-data; boundary="+boundary})
    with urllib.request.urlopen(req,timeout=20) as res: return json.load(res)


assert request('/users/me', token=owner)['avatar_upload_enabled']
first = upload('/users/me/avatar',owner)['avatar']
second = upload('/users/me/avatar',owner)['avatar']
assert first != second
parts = urllib.parse.urlsplit(base)
origin = parts.scheme + '://' + parts.netloc
with urllib.request.urlopen(origin+second,timeout=20) as res:
    assert res.headers['Content-Type'].startswith('image/jpeg') and res.read(2) == b"\xff\xd8"
try:
    urllib.request.urlopen(origin+first,timeout=20)
except urllib.error.HTTPError as e:
    assert e.code == 404
else:
    raise AssertionError('Replaced avatar remains accessible')
group_path = '/groups/'+str(g['id'])+'/avatar'
for token in (owner, user):
    try:
        upload(group_path,token)
    except urllib.error.HTTPError as e:
        assert e.code == 404
    else:
        raise AssertionError('Group avatar upload should not exist')
info = request('/notifications/'+str(received['id']),token=user)
assert info['card']['actor_avatar'] == second and 'avatar' not in info['group']
try:
    request('/notifications/'+str(received['id']),token=owner)
except urllib.error.HTTPError as e:
    assert e.code == 404
else:
    raise AssertionError('Notification detail leaked')
print('PASS: personal avatar upload, image fetch, replacement cleanup, disabled group avatar endpoint and fresh notification detail')
