import { createApp, h, ref } from 'vue'
import './style.css'
import App from './App.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'

// Last line of defence: an error outside the sections' own boundaries
// (header, rail, the page's own logic) shows a message instead of a blank
// page, and the dashboard is started again every 15 s.
const retry = ref(0)
setInterval(() => retry.value++, 15_000)

createApp({
  render: () => h(ErrorBoundary, { label: 'the dashboard', resetKey: retry.value, page: true }, () => h(App)),
}).mount('#app')
