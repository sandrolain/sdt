import { html, css, LitElement } from 'lit'
import { customElement, query, state } from 'lit/decorators.js'
import "./wasm_exec.js"
import wasm from "./sdt.wasm?url";
import "./main.css";

interface Preset {
  name: string;
  command: string;
  input: boolean;
}

const presets: Preset[] = [{
  name: "Custom",
  command: "",
  input: true
}, {
  name: "Random bytes Base64",
  command: "sdt pipe - bytes - b64",
  input: false
}, {
  name: "Random bytes Hexadecimal",
  command: "sdt pipe - bytes - hex",
  input: false
}, {
  name: "Base64 Encode",
  command: "sdt b64",
  input: true
}, {
  name: "Cthulhu",
  command: "sdt cthulhu",
  input: false
}];

@customElement('sdt-app')
export class SdtApp extends LitElement {
  static styles = css`
    :host {
      box-sizing: border-box;
      width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      gap: 16px;
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
      gap: 16px;
    }
    #top > div {
      display: flex;
      flex-direction: column;
    }
    #top > div:nth-of-type(2) {
      flex: 1;
    }

    select, input, textarea, button {
      font-family: 'Courier New', Courier, monospace !important;
      background: var(--bg-color);
      color: var(--tx-color);
      border: 1px solid var(--tx-color);
      padding: 8px;
      font-size: 16px;
      height: 34px;
      line-height: 1em;
      box-sizing: border-box;
    }
    button {
      background: var(--tx-color);
      color: var(--bg-color);
      cursor: pointer;
    }

    textarea {
      width: 100%;
      height: 100%;
      resize: none;
    }

    #bottom {
      width: 100%;
      display: flex;
      gap: 16px;
      flex: 1;
    }
    #bottom > div {
      flex: 1;
      display: flex;
      flex-direction: column;
    }

  `;

  @query("#input")
  private $input!: HTMLTextAreaElement;

  @query("#preset")
  private $preset!: HTMLSelectElement;

  @query("#command")
  private $command!: HTMLInputElement;

  @query("#output")
  private $output!: HTMLTextAreaElement;

  @state()
  private hideInput: boolean = false;

  render() {
    return html`
      <div id="top">
        <h1>Smart Developer Tools ^(;,;)^</h1>
        <div>
          <label for="preset">Preset</label>
          <select id="preset" @change=${this.onPresetChange}>${presets.map((p) => html`<option value=${p.command}>${p.name}</option>`)}</select>
        </div>
        <div>
          <label for="command">Command</label>
          <input type="text" id="command" autocomplete="off" autocapitalize="off" spellcheck="false" />
        </div>
        <div>
          <label for="execute">&nbsp;</label>
          <button type="button" id="execute" @click=${this.onExecute}>Execute</button>
        </div>
      </div>
      <div id="bottom">
        ${this.hideInput ? null : html`
        <div id="input-wrp">
          <label for="input">Input</label>
          <textarea id="input" autocomplete="off" autocapitalize="off" spellcheck="false"></textarea>
        </div>
        `}
        <div id="output-wrp">
          <label for="output">Output</label>
          <textarea id="output" readonly></textarea>
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
    const decoder = new TextDecoder("utf-8");
    (window as any).fs.writeSync = (_: number, buf: BufferSource) => {
      this.outputBuf += decoder.decode(buf);
      this.applyOutput();
      return (buf as any).length;
    };
  }

  private applyOutput(): void {
    this.$output.value = this.outputBuf;
  }

  private onPresetChange() {
    const value = this.$preset.value;
    this.$command.value = value;
    const preset = presets.find((p) => (p.command === value));
    this.hideInput = !preset?.input;
    this.outputBuf = "";
    this.applyOutput();
  }

  private async onExecute() {
    this.outputBuf = "";
    const wa = await WebAssembly.instantiate(this.wasm, this.Go.importObject);
    const input = this.$input?.value ?? "";
    const args = this.$command.value.split(/\s+/);
    if(input) {
      args.push(input);
    }
    this.Go.argv = args;
    this.Go.run(wa.instance);
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sdt-app': SdtApp
  }
}
