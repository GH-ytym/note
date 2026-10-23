package notification

import _ "embed"

//go:embed lua/init_snapshot.lua
var initSnapshotLua string

//go:embed lua/apply_change.lua
var applyChangeLua string

//go:embed lua/begin_rebuild.lua
var beginRebuildLua string

//go:embed lua/end_rebuild.lua
var endRebuildLua string

//go:embed lua/install_snapshot.lua
var installSnapshotLua string

//go:embed lua/publish_notice.lua
var publishNoticeLua string
