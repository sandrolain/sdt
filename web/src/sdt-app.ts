import { html, css, LitElement, PropertyValueMap } from 'lit'
import { customElement, property, query, state } from 'lit/decorators.js'
import "./wasm_exec.js"
import wasm from "./sdt.wasm?url";

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
      display: block;
      border: solid 1px gray;
      padding: 16px;
      max-width: 800px;
    }
  `;

  @query("#input")
  private $input: HTMLTextAreaElement;

  @query("#preset")
  private $preset: HTMLSelectElement;

  @query("#command")
  private $command: HTMLInputElement;

  @query("#output")
  private $output: HTMLTextAreaElement;

  @state()
  private hideInput: boolean = false;

  render() {
    return html`
      ${this.hideInput ? null : html`
      <div id="input-wrp">
        <textarea id="input"></textarea>
      </div>
      `}
      <div id="input-wrp">
        <select id="preset" @change=${this.onPresetChange}>${presets.map((p) => html`<option value=${p.command}>${p.name}</option>`)}</select>
        <input type="text" id="command" />
        <button type="button" id="execute" @click=${this.onExecute}>Execute</button>
      </div>
      <div id="input-wrp">
        <textarea id="output" readonly></textarea>
      </div>
    `;
  }

  private Go: Go;
  private outputBuf: string = '';

  protected async firstUpdated(): Promise<void> {
    this.Go = new Go();


    const decoder = new TextDecoder("utf-8");
    (window as any).fs.writeSync = (fd, buf) => {
      this.outputBuf += decoder.decode(buf);
      this.$output.value = this.outputBuf;
      return buf.length;
    };
  }

  private onPresetChange() {
    const value = this.$preset.value;
    this.$command.value = value;
    const preset = presets.find((p) => (p.command === value));
    this.hideInput = !preset?.input;
  }

  private async onExecute() {
    this.outputBuf = "";
    const wa = await WebAssembly.instantiateStreaming(fetch(wasm), this.Go.importObject);
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
