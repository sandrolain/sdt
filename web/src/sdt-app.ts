import { html, css, LitElement } from 'lit'
import { customElement, query, state } from 'lit/decorators.js'
import { classMap } from 'lit/directives/class-map.js'
import "./wasm_exec.js"
import wasm from "./sdt.wasm?url";
import "./main.css";
import { presets } from './presets';
import copy from "copy-to-clipboard";
import { split } from "shellwords";

@customElement('sdt-app')
export class SdtApp extends LitElement {
  static styles = css`
    :host {
      box-sizing: border-box;
      width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 16px;
      font-family: 'Courier New', Courier, monospace !important;
    }

    h1 {
      font-size: 1em;
      margin: 0;
      text-align: center;
      line-height: 34px;
      padding-top: 16px;
    }

    #top {
      display: flex;
      gap: 8px;
    }
    #top > div {
      display: flex;
      flex-direction: column;
    }
    #top > div:nth-of-type(2) {
      flex: 1;
    }

    select, input:not([type="file"], [type="checkbox"]), textarea, button,
    input::-webkit-file-upload-button {
      font-family: 'Courier New', Courier, monospace !important;
      background: var(--bg-color);
      color: inherit;
      border: 1px solid currentColor;
      padding: 8px;
      font-size: 16px;
      height: 34px;
      line-height: 1em;
      box-sizing: border-box;
      border-radius: var(--radius);
    }
    button,
    input::-webkit-file-upload-button {
      background: var(--tx-color);
      color: var(--bg-color);
      cursor: pointer;
      border-radius: var(--radius);
      border: 1px solid var(--tx-color);
      box-sizing: border-box;
    }
    button:hover,
    input::-webkit-file-upload-button:hover {
      background: var(--tx-color-2);
      border-color: var(--tx-color-2);
    }
    input[type="file"] {
      font-family: inherit;
    }
    input::-webkit-file-upload-button {
      height: 18px;
      line-height: 18px;
      font-size: 14px;
      padding: 0px 2px;
    }

    textarea {
      width: 100%;
      height: 100%;
      resize: none;
    }

    #bottom {
      width: 100%;
      display: flex;
      gap: 8px;
      flex: 1;
      color: var(--tx-color);
    }
    #bottom > div {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    #bottom > div > div {
      flex: 1;
      display: flex;
      gap: 8px;
    }
    #bottom.error #output-wrp {
      color: var(--er-color);
    }
    #input-head {
      display: flex;
    }
    #input-head > * {
      flex: 1;
    }

    a {
      color: inherit;
      text-decoration: none;
    }
    a:hover {
      text-decoration: underline;
    }

  `;

  @query("#input")
  private $input!: HTMLTextAreaElement;

  @query("#file")
  private $file!: HTMLInputElement;

  @query("#b64")
  private $b64!: HTMLInputElement;

  @query("#preset")
  private $preset!: HTMLSelectElement;

  @query("#command")
  private $command!: HTMLInputElement;

  @query("#output")
  private $output!: HTMLTextAreaElement;

  @state()
  private hideInput: boolean = false;

  @state()
  private error: boolean = false;

  render() {
    return html`
      <div id="top">
        <h1><a href="https://github.com/sandrolain/sdt">Smart Developer Tools ^(;,;)^</a></h1>
        <div>
          <label for="preset">Preset</label>
          <select id="preset" @change=${this.onPresetChange}>${presets.map((p) => html`<option value=${p.command}>${p.name}</option>`)}</select>
        </div>
        <div>
          <label for="command">Command</label>
          <input type="text" id="command" autocomplete="off" autocapitalize="off" spellcheck="false" @keydown=${(event: KeyboardEvent) => {
            if(event.code === "Enter") {
              this.onExecute();
            }
          }}  />
        </div>
        <div>
          <label for="execute">&nbsp;</label>
          <button type="button" id="execute" @click=${this.onExecute}>Execute</button>
        </div>
      </div>
      <div id="bottom" class=${classMap({"error": this.error})}>
        ${this.hideInput ? null : html`
        <div id="input-wrp">
          <div id="input-head">
            <label for="input">Input</label>
            <input type="file" id="file" @change=${this.onFileSelect} />
            <input type="checkbox" id="b64" />
          </div>
          <textarea id="input" autocomplete="off" autocapitalize="off" spellcheck="false"></textarea>
        </div>
        `}
        <div id="output-wrp">
          <label for="output">Output</label>
          <div id="output-cnt">
            <textarea id="output" readonly></textarea>
            <button id="copy" @click=${() => {
              copy(this.$output.value);
            }}>Copy</button>
          </div>
        </div>
      </div>
    `;
  }

  private Go!: Go;
  private outputBuf: string = '';
  private wasm!: ArrayBuffer;

  protected async firstUpdated(): Promise<void> {
    this.wasm = await (await fetch(wasm)).arrayBuffer();
    this.Go = new Go();
    this.Go.exit = (code) => {
      if(code > 0) {
        this.error = true;
      }
      this.applyOutput();
    };
    const decoder = new TextDecoder("utf-8");
    (window as any).fs.writeSync = (_: number, buf: BufferSource) => {
      this.outputBuf += decoder.decode(buf);
      return (buf as any).length;
    };
  }

  private callback!: ((out: string) => void) | undefined;

  private async execute(argv: string[], callback?: (out: string) => void): Promise<void> {
    this.callback = callback;
    const wa = await WebAssembly.instantiate(this.wasm, this.Go.importObject);
    this.Go.argv = argv;
    this.Go.run(wa.instance);
  }

  private applyOutput(): void {
    if(this.callback) {
      this.callback(this.outputBuf);
      this.callback = undefined;
    } else {
      this.$output.value = this.outputBuf;
    }
  }

  private onPresetChange() {
    const command = this.$preset.value;
    const preset  = presets.find((p) => (p.command === command));
    this.$command.value = command;
    this.hideInput      = !preset?.input;
    this.reset();
  }

  private reset() {
    this.outputBuf   = "";
    this.error       = false;
    this.$file.value = "";
    this.applyOutput();
  }

  private async onExecute() {
    this.outputBuf = "";
    this.error = false;

    const input     = (this.$input?.value ?? "").trim();
    const b64       = this.$b64?.checked;
    const cmdString = this.$command.value;
    const args      = split(cmdString);
    if(args[0] === "sdt") {
      args.splice(0, 1)
    }
    if(input) {
      if(args[0] === ":") {
        if(b64) {
          args.splice(0, 0, ":", "--inb64", input)
        } else {
          args.splice(0, 0, ":", "--input", input)
        }
      } else {
        if(b64) {
          args.push("--inb64", input)
        } else {
          args.push("--input", input)
        }
      }
    }
    args.unshift("sdt")
    await this.execute(args);
  }

  private MAX_FILE_SIZE = (4096 + 8192) / 2;

  private onFileSelect() {
    const files = Array.from(this.$file?.files ?? []);
    const file = files[0];

    if(!file) {
      return;
    }

    if (file.size > this.MAX_FILE_SIZE) {
      this.error = true;
      this.outputBuf =
        `File size too large: ${file.size} bytes\n` +
        `Max file size:       ${this.MAX_FILE_SIZE} bytes`;
      this.applyOutput();
      return;
    }

    const reader = new FileReader();

    reader.onload = () => {
      const uri = reader.result as string;
      const b64 = uri.substring(uri.indexOf(",") + 1);
      this.$input.value = b64;
      this.$b64.checked = true;
    };

    reader.onerror = () =>  {
      this.error = true;
    };

    reader.readAsDataURL(file);
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sdt-app': SdtApp
  }
}
