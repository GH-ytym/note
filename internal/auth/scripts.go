package auth

import _ "embed"

//go:embed lua/replace_refresh.lua
var replaceRefreshLua string

//go:embed lua/rotate_refresh.lua
var rotateRefreshLua string
