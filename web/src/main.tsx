import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
// self-hosted fonts (Q5): the declared IBM Plex Sans and the Material Symbols
// icon font, both bundled so web/dist - and the embedded sdtviewer - carry them
import "@fontsource-variable/ibm-plex-sans";
import "@fontsource-variable/material-symbols-outlined";
import "./styles/tokens.css";
import "./styles/index.css";
import "dockview-react/dist/styles/dockview.css";
import "./styles/dockview.css";
import { App } from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
