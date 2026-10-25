const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "stream-transport.js"), "utf8");
const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, "../../testdata/pty-recipe.json"), "utf8"));

function transport(context = {}) {
  const scope = {URL, ...context};
  vm.runInNewContext(source, scope);
  return scope.StreamTransport;
}

test("a fixture terminal recipe maps authorization and frames", async () => {
  const requests = [];
  const stream = transport({
    async fetch(url, options) {
      requests.push({url, options});
      return {
        ok: true,
        status: 200,
        headers: {get() { return "application/json"; }},
        async json() { return {result: "OK", allowed: true}; },
      };
    },
    WebSocket: {OPEN: 1},
  });
  assert.equal(stream.validate(fixture, fixture.origin), true);
  await stream.authorize(fixture);
  assert.equal(requests[0].url, fixture.authorization.url);
  assert.equal(requests[0].options.credentials, "include");

  const sent = [];
  const socket = {readyState: 1, send(data) { sent.push(JSON.parse(data)); }};
  assert.equal(stream.send(socket, fixture, {type: "stdin", data: "pwd\n"}), true);
  assert.equal(stream.send(socket, fixture, {type: "resize", cols: 100, rows: 30}), true);
  assert.deepEqual(sent, [
    {action: "input", payload: "pwd\n"},
    {action: "window", columns: 100, lines: 30},
  ]);
  assert.deepEqual(JSON.parse(JSON.stringify(stream.decode(fixture, JSON.stringify({action: "output", payload: "ok\n"})))),
    {type: "stdout", data: "ok\n"});
  assert.deepEqual(JSON.parse(JSON.stringify(stream.decode(fixture, JSON.stringify({action: "metadata", payload: "ready"})))),
    {type: "header", data: "ready"});
});

test("a terminal recipe cannot redirect authorization or sockets to another origin", () => {
  const stream = transport();
  const recipe = structuredClone(fixture);
  recipe.authorization.url = "https://other.example.test/api/permission";
  assert.equal(stream.validate(recipe, fixture.origin), false);
  recipe.authorization.url = fixture.authorization.url;
  recipe.socketUrl = "wss://other.example.test/api/terminal/stream";
  assert.equal(stream.validate(recipe, fixture.origin), false);
});

test("authorization denial stays distinct from a malformed terminal frame", async () => {
  const stream = transport({
    async fetch() {
      return {
        ok: true, status: 200,
        headers: {get() { return "application/json"; }},
        async json() { return {result: "OK", allowed: false}; },
      };
    },
  });
  await assert.rejects(stream.authorize(fixture), stream.AuthorizationRequiredError);
  assert.throws(() => stream.decode(fixture, "not-json"), /invalid terminal frame/);
});
