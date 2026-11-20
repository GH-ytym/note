const { build } = require('./package.json');

module.exports = {
  ...build,
  appId: 'cn.note.calendar.server',
  productName: 'Note Server',
  directories: { output: 'release/server' },
  extraResources: [{ from: 'resources/server.json', to: 'server.json' }],
  win: { ...build.win, extraResources: [] },
  mac: { ...build.mac, extraResources: [] },
  nsis: { ...build.nsis, artifactName: 'Note-Server-Setup-${version}.${ext}' },
  portable: { ...build.portable, artifactName: 'Note-Server-${version}.${ext}' },
};
