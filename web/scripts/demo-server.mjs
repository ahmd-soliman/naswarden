// The UI plus a fake backend, shared by `npm run demo` and `npm run screenshots`:
// Vite serves the UI, and /version and a push-only /ws are answered here, so any browser can open it with no TrueNAS and no mocks.
// (The dashboard only listens on the socket, so no ws package is needed.)

import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { createServer } from 'vite'
import { demoState } from './demo-state.mjs'

const web = fileURLToPath(new URL('..', import.meta.url))

// One unmasked server->client text frame.
function frame(text) {
  const payload = Buffer.from(text)
  const n = payload.length
  if (n < 126) return Buffer.concat([Buffer.from([0x81, n]), payload])
  if (n < 65536) return Buffer.concat([Buffer.from([0x81, 126, n >> 8, n & 255]), payload])
  const head = Buffer.alloc(10)
  head[0] = 0x81
  head[1] = 127
  head.writeBigUInt64BE(BigInt(n), 2)
  return Buffer.concat([head, payload])
}

// A control frame (close / pong) from the server; payload under 126 bytes.
function control(opcode, payload = Buffer.alloc(0)) {
  return Buffer.concat([Buffer.from([0x80 | opcode, payload.length]), payload])
}

// Reads the (masked) frames a browser sends. Only close and ping matter here:
// a close is answered and ends the socket, a ping gets a pong. Anything else is ignored.
function readFrames(socket, buffered) {
  let buf = buffered
  for (;;) {
    if (buf.length < 2) break
    const opcode = buf[0] & 0x0f
    const masked = (buf[1] & 0x80) !== 0
    let len = buf[1] & 0x7f
    let offset = 2
    if (len === 126) {
      if (buf.length < 4) break
      len = buf.readUInt16BE(2)
      offset = 4
    } else if (len === 127) {
      if (buf.length < 10) break
      len = Number(buf.readBigUInt64BE(2))
      offset = 10
    }
    const end = offset + (masked ? 4 : 0) + len
    if (buf.length < end) break
    const payload = Buffer.from(buf.subarray(end - len, end))
    if (masked) {
      const key = buf.subarray(end - len - 4, end - len)
      for (let i = 0; i < payload.length; i++) payload[i] ^= key[i % 4]
    }
    buf = buf.subarray(end)
    if (opcode === 0x8) {
      socket.end(control(0x8, payload.subarray(0, 2)))
      return Buffer.alloc(0)
    }
    if (opcode === 0x9 && payload.length < 126) socket.write(control(0xa, payload))
  }
  return buf
}

// startDemo({ port, version }) -> { url, close }. port 0 picks a free one.
export async function startDemo({ port = 0, version = 'demo' } = {}) {
  const build = { version, commit: 'demo', date: new Date().toISOString() }
  const timers = new Set()
  const sockets = new Set()
  const server = await createServer({
    root: web,
    configFile: false, // vite.config.ts proxies /ws to a real backend; the demo answers it itself
    logLevel: 'warn',
    server: { port, strictPort: false },
    plugins: [
      vue(),
      {
        name: 'demo-backend',
        configureServer(vite) {
          vite.middlewares.use('/version', (_req, res) => {
            res.setHeader('content-type', 'application/json')
            res.end(JSON.stringify(build))
          })
          vite.httpServer.on('upgrade', (req, socket) => {
            if (new URL(req.url, 'http://x').pathname !== '/ws') return
            if (req.headers.upgrade?.toLowerCase() !== 'websocket' || !req.headers['sec-websocket-key']) {
              socket.end('HTTP/1.1 400 Bad Request\r\n\r\n')
              return
            }
            const accept = createHash('sha1')
              .update(req.headers['sec-websocket-key'] + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11')
              .digest('base64')
            socket.write(
              'HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n' +
                `Sec-WebSocket-Accept: ${accept}\r\n\r\n`,
            )
            sockets.add(socket)
            // skip a push while the previous one is still waiting to be sent
            const send = () => socket.writable && !socket.writableNeedDrain && socket.write(frame(JSON.stringify(demoState())))
            send()
            const timer = setInterval(send, 2000)
            timers.add(timer)
            const stop = () => {
              clearInterval(timer)
              timers.delete(timer)
              sockets.delete(socket)
            }
            socket.on('close', stop)
            socket.on('error', stop)
            let pending = Buffer.alloc(0)
            socket.on('data', (chunk) => {
              pending = readFrames(socket, Buffer.concat([pending, chunk]))
            })
          })
        },
      },
    ],
  })
  await server.listen()
  return {
    url: server.resolvedUrls.local[0],
    async close() {
      timers.forEach(clearInterval)
      sockets.forEach((s) => s.destroy())
      await server.close()
    },
  }
}
