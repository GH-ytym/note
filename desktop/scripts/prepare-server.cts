import fs = require("node:fs");
import path = require("node:path");
import { serverURL } from "../server-config.cjs";

const url = serverURL(process.env.NOTE_SERVER_URL);
const dir = path.resolve(__dirname, "../../resources");
fs.mkdirSync(dir, { recursive: true });
fs.writeFileSync(path.join(dir, "server.json"), JSON.stringify({ url }, null, 2) + "\n");
console.log(`服务器版客户端：${url}`);
