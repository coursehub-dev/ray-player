import { mount } from "svelte";
import App from "./App.svelte";
import "./style.css";

const target = document.getElementById("app");
if (!target) {
	throw new Error("#app root element not found");
}

const app = mount(App, { target });

export default app;
