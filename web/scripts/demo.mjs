// Serves the UI with demo data, no TrueNAS or backend. Open the printed URL in
// any browser:
//
//   npm run demo

import { startDemo } from './demo-server.mjs'

const demo = await startDemo({ port: 5199 })
console.log(`Demo running at ${demo.url}  (Ctrl+C to stop)`)
process.on('SIGINT', async () => {
  await demo.close()
  process.exit(0)
})
