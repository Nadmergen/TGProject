import './style.css'
import App from './App.svelte'
import { theme } from './stores'

const app = new App({
  target: document.getElementById('app')
})

theme.subscribe((t) => {
  document.documentElement.dataset.theme = t
})

export default app
