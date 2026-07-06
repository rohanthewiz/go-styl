/* worker.js — hosts the Stylus compiler off the main thread.
 *
 * Why a worker: goStyl.compile is synchronous, and pathological input
 * (deep mixin recursion, huge range loops) can spin for a long time. A
 * wasm-side timeout can't save the page — Go's timer goroutine never gets
 * scheduled while a goroutine spins without yielding in browser wasm
 * (proven on the element playground, same architecture). In a worker the
 * main thread stays responsive and, on timeout, runner.js terminate()s
 * this worker and spawns a fresh one.
 *
 * Protocol (see runner.js):
 *   -> {id, src, opts}                        compile request
 *   <- {type:'ready', version, examples}      wasm instantiated, goStyl installed
 *   <- {type:'result', id, r}                 r = goStyl.compile(src, opts)
 *   <- {type:'fatal', error}                  wasm failed to load
 */
importScripts('wasm_exec.js');

var go = new Go();

self.goStylReady = function () {
	postMessage({
		type: 'ready',
		version: (self.goStyl && self.goStyl.version) || 'dev',
		examples: self.goStyl ? self.goStyl.examples() : [],
	});
};

// Prefer streaming; fall back for servers that mislabel .wasm's MIME type.
var load = WebAssembly.instantiateStreaming
	? WebAssembly.instantiateStreaming(fetch('styl.wasm'), go.importObject)
		.catch(function () {
			return fetch('styl.wasm').then(function (r) { return r.arrayBuffer(); })
				.then(function (buf) { return WebAssembly.instantiate(buf, go.importObject); });
		})
	: fetch('styl.wasm').then(function (r) { return r.arrayBuffer(); })
		.then(function (buf) { return WebAssembly.instantiate(buf, go.importObject); });

load.then(function (r) { go.run(r.instance); })
	.catch(function (err) { postMessage({ type: 'fatal', error: String(err) }); });

self.onmessage = function (e) {
	// Synchronous compile: further messages queue in the worker's event loop
	// (FIFO), so results always arrive in send order.
	postMessage({ type: 'result', id: e.data.id, r: self.goStyl.compile(e.data.src, e.data.opts) });
};
