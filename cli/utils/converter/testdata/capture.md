---
url: https://www.assistant-ui.com/safe-content-frame
title: Safe Content Frame · assistant-ui
saved_at: "2026-11-01T16:19:32Z"
description: Sandboxes for HTML. Render MCP Apps and Generative UI in isolated iframes with their own origins.
---

# Safe Content Frame · assistant-ui

## Sandboxes for HTML

Every render gets its own iframe and origin, so the HTML inside can’t reach your app’s DOM, cookies, or storage.

## Use cases

## Compared with iframe sandbox

Both sandbox your HTML. What differs is the origin it runs on.

### iframe sandbox

origin: null

Runs

- Sandboxed HTML, CSS, and JS

Storage

- document.cookie
- window.localStorage
- window.sessionStorage
- window.indexedDB
- window.caches
- navigator.storage

Workers and locks

- window.SharedWorker
- navigator.serviceWorker
- navigator.locks

Messaging and network

- window.postMessage: only \* allowed
- CORS: only \* allowed

### Safe Content Frame

origin: https://<hash>.scf.auiusercontent.com

Runs

- Sandboxed HTML, CSS, and JS

Storage

- document.cookie
- window.localStorage
- window.sessionStorage
- window.indexedDB
- window.caches
- navigator.storage

Workers and locks

- window.SharedWorker
- navigator.serviceWorker
- navigator.locks

Messaging and network

- window.postMessage: scoped to its origin
- CORS: allowlist its origin

## How it works

scf.auiusercontent.com is on the [Public Suffix List](https://publicsuffix.org/), so every subdomain is treated as its own domain.

domain: example.com

- docs.example.com
- app.example.com
- cdn.example.com

shared cookies and storage

- domain: k3f9……

k3f9…-h184756.scf.auiusercontent.comown storage

- domain: 0qzm……

0qzm…-h184756.scf.auiusercontent.comown storage

- domain: x81d……

x81d…-h184756.scf.auiusercontent.comown storage

Based on the design Google published for [SafeContentFrame](https://bughunters.google.com/blog/beyond-sandbox-domains-rendering-untrusted-web-content-with-safecontentframe).

## Features

### A domain per render

Each render gets a hashed hostname on a public suffix.

renderHtml()

### Verified hand-off

The frame only accepts content from your page’s origin.

?origin=

### Scoped messaging

Send messages addressed to the frame’s exact origin.

sendMessage()

### HTML, PDFs, and more

Render HTML strings, PDFs, or any MIME type.

renderPdf(), renderRaw()

### Load and error states

Wait for the frame to load, with typed errors when it doesn’t.

fullyLoadedPromiseWithTimeout()

### Cancel and clean up

Abort a pending render, or remove the frame when you’re done.

signal, dispose()

### Shadow DOM

Mount the iframe inside a closed shadow root.

useShadowDom

### Opt-in caching

Reuse a domain and its HTTP cache for identical content.

enableBrowserCaching

### No dependencies

Plain JavaScript that works with any framework.

safe-content-frame