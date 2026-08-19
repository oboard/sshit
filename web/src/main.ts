import App from "./App.svelte";
import "xterm/css/xterm.css";
import "./style.css";

const app = new App({
  target: document.getElementById("app")!,
});

export default app;
