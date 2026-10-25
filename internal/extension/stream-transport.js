/* Browser-side executor for CLI-owned, declarative JSON PTY stream recipes. */
(() => {
  "use strict";

  class AuthorizationRequiredError extends Error {}
  const own = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
  const field = (value) => typeof value === "string" && /^[A-Za-z][A-Za-z0-9_]{0,31}$/.test(value);
  const label = (value) => typeof value === "string" && value.length > 0 && value.length <= 64;

  function sameOriginURL(value, origin, scheme) {
    if (typeof value !== "string" || value.length > 8192) return false;
    try {
      const url = new URL(value);
      const allowed = new URL(origin);
      return url.protocol === scheme && url.host === allowed.host &&
        !url.username && !url.password && !url.hash && url.pathname.startsWith("/");
    } catch (_) {
      return false;
    }
  }

  function validate(recipe, allowedOrigin) {
    let origin;
    try { origin = new URL(allowedOrigin); } catch (_) { return false; }
    if (origin.origin !== allowedOrigin ||
        !(origin.protocol === "https:" || origin.protocol === "http:" &&
          ["127.0.0.1", "localhost", "[::1]"].includes(origin.hostname))) return false;
    if (!recipe || typeof recipe !== "object" || Array.isArray(recipe) ||
        JSON.stringify(recipe).length > 16384 || recipe.version !== 1 ||
        recipe.origin !== allowedOrigin ||
        !sameOriginURL(recipe.pageUrl, allowedOrigin, new URL(allowedOrigin).protocol) ||
        !sameOriginURL(recipe.authorization?.url, allowedOrigin, new URL(allowedOrigin).protocol) ||
        !sameOriginURL(recipe.socketUrl, allowedOrigin,
          new URL(allowedOrigin).protocol === "https:" ? "wss:" : "ws:")) return false;
    const auth = recipe.authorization;
    const frame = recipe.frame;
    if (!auth || !frame || !field(auth.statusField) || !field(auth.accessField) ||
        auth.statusField === auth.accessField || !label(auth.successValue) ||
        !label(auth.loginValue) || typeof auth.accessGranted !== "boolean") return false;
    for (const key of ["operationField", "dataField", "colsField", "rowsField"]) {
      if (!field(frame[key]) || frame[key] === "__proto__") return false;
    }
    if (new Set([frame.operationField, frame.dataField, frame.colsField, frame.rowsField]).size !== 4) return false;
    for (const key of ["stdin", "stdout", "header", "resize"]) {
      if (!label(frame[key])) return false;
    }
    return new Set([frame.stdin, frame.stdout, frame.header, frame.resize]).size === 4;
  }

  async function authorize(recipe) {
    const response = await fetch(recipe.authorization.url, {
      method: "GET", credentials: "include",
      headers: {Accept: "application/json, text/plain, */*"},
    });
    const contentType = response.headers.get("content-type") || "";
    if (!contentType.includes("json")) {
      throw new Error(`terminal authorization returned a non-JSON response (HTTP ${response.status})`);
    }
    const body = await response.json();
    if (!response.ok) throw new Error(`terminal authorization failed with HTTP ${response.status}`);
    const auth = recipe.authorization;
    if (!body || typeof body !== "object" || !own(body, auth.statusField)) {
      throw new Error("terminal authorization response is invalid");
    }
    if (body[auth.statusField] === auth.loginValue) {
      throw new Error("terminal session is not logged in; refresh the site page and retry");
    }
    if (body[auth.statusField] !== auth.successValue) {
      throw new Error("terminal authorization check failed");
    }
    if (!own(body, auth.accessField) || body[auth.accessField] !== auth.accessGranted) {
      throw new AuthorizationRequiredError("terminal access is not authorized for this target");
    }
  }

  function send(socket, recipe, message) {
    if (socket?.readyState !== WebSocket.OPEN) return false;
    const frame = recipe.frame;
    let outgoing;
    if (message.type === "stdin" && typeof message.data === "string" && message.data.length <= 1048576) {
      outgoing = {[frame.operationField]: frame.stdin, [frame.dataField]: message.data};
    } else if (message.type === "resize" && Number.isInteger(message.cols) && Number.isInteger(message.rows) && message.cols > 0 && message.cols <= 10000 && message.rows > 0 && message.rows <= 10000) {
      outgoing = {[frame.operationField]: frame.resize, [frame.colsField]: message.cols, [frame.rowsField]: message.rows};
    } else {
      return false;
    }
    socket.send(JSON.stringify(outgoing));
    return true;
  }

  function decode(recipe, data) {
    if (typeof data !== "string" || data.length > 1048576) {
      throw new Error("unsupported or oversized terminal frame");
    }
    let message;
    try { message = JSON.parse(data); }
    catch (_) { throw new Error("invalid terminal frame"); }
    if (!message || typeof message !== "object") throw new Error("invalid terminal frame");
    const frame = recipe.frame;
    const operation = message[frame.operationField];
    if (operation === frame.stdout || operation === frame.header) {
      if (typeof message[frame.dataField] !== "string") throw new Error("invalid terminal frame");
      return {type: operation === frame.stdout ? "stdout" : "header", data: message[frame.dataField]};
    }
    return null;
  }

  function startHeartbeat(socket, recipe, lastInputAt, onInput) {
    return setInterval(() => {
      if (socket.readyState === WebSocket.OPEN && Date.now() - lastInputAt() > 30000) {
        send(socket, recipe, {type: "stdin", data: "\0"});
        onInput();
      }
    }, 10000);
  }

  globalThis.StreamTransport = Object.freeze({
    AuthorizationRequiredError, validate, authorize, send, decode, startHeartbeat,
  });
})();
