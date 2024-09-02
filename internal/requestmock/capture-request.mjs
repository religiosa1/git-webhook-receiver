//@ts-check

/** Simple nodejs script for capturing and dumping requests for usage in our tests */

import { createServer } from "node:http";
import { writeFile } from "node:fs/promises";

const PORT = 9090;

const server = createServer(async (req, res) => {
  try {
    console.log("Capturing request ", req.socket.remoteAddress, req.url);
    const body = await getBody(req);

    const reqDump = {
      url: req.url,
      method: req.method,
      headers: req.headers,
      body,
    };

    await writeFile(getFileName(), JSON.stringify(reqDump, undefined, 2));
    console.log("Done");
    res.writeHead(200, { "Content-Type": "application/json" }).end(
      JSON.stringify({
        ok: true,
      })
    );
  } catch (err) {
    console.error("Error", err);
    res.writeHead(500, { "Content-Type": "application/json" }).end(
      JSON.stringify({
        ok: false,
        error: String(err),
      })
    );
  }
});

/**
 * @param {import("node:http").IncomingMessage} req
 * @returns {Promise<string>} response
 */
function getBody(req) {
  return new Promise((res, rej) => {
    let body;

    req.on("data", handleData);
    req.once("end", handleEnd);
    req.once("error", handleError);

    function handleData(chunk) {
      body ??= "";
      body += chunk.toString();
    }

    function handleEnd() {
      req.off("data", handleData);
      req.off("error", handleError);
      res(body);
    }

    function handleError(err) {
      req.off("data", handleData);
      req.off("end", handleEnd);
      rej(err);
    }
  });
}

function getFileName() {
  const d = new Date();
  const yyyy = d.getFullYear();
  const MM = (d.getMonth() + 1).toString().padStart(2, "0");
  const dd = d.getDate().toString().padStart(2, "0");
  const hh = d.getHours().toString().padStart(2, "0");
  const mm = d.getMinutes().toString().padStart(2, "0");
  const ss = d.getSeconds().toString().padStart(2, "0");
  const ms = d.getMilliseconds().toString().padStart(3, "0");

  return `${yyyy}-${MM}-${dd}_${hh}${mm}${ss}_${ms}.json`;
}

server.listen(PORT, undefined, () => {
  console.log("Server is listening on port ", PORT);
});