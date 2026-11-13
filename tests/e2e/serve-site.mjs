// Serves a folder over HTTPS on this machine, so the installer can be run
// against a release that is staged in site/public but not yet published.
// The installer only talks HTTPS, which is why this is not plain HTTP.
//
//   node serve-site.mjs <dir> <port> <key.pem> <cert.pem>
import https from "node:https";
import { createReadStream, readFileSync, statSync } from "node:fs";
import path from "node:path";

const [dir, port, key, cert] = process.argv.slice(2);
const root = path.resolve(dir);
https
  .createServer({ key: readFileSync(key), cert: readFileSync(cert) }, (req, res) => {
    const file = path.normalize(path.join(root, decodeURIComponent(new URL(req.url, "https://localhost").pathname)));
    let info = null;
    if (file.startsWith(root + path.sep)) {
      try {
        info = statSync(file);
      } catch {
        info = null;
      }
    }
    if (!info || !info.isFile()) {
      res.writeHead(404).end("not found\n");
      return;
    }
    res.writeHead(200, { "content-length": info.size });
    createReadStream(file).pipe(res);
  })
  .listen(Number(port), "127.0.0.1", () => console.log(`serving ${root} at https://localhost:${port}`));
