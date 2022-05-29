
interface Preset {
  name: string;
  command: string;
  input: boolean;
  inputAsFlag?: string;
  flags?: string[];
}

export const presets: Preset[] = [{
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
}, {
  name: "JSON => YAML",
  command: "sdt conv -a json -b yaml",
  input: true
}, {
  name: "JSON => TOML",
  command: "sdt conv -a json -b toml",
  input: true
}, {
  name: "JSON => QUERY",
  command: "sdt conv -a json -b query",
  input: true
}, {
  name: "JSON => CSV",
  command: "sdt conv -a json -b csv",
  input: true
}, {
  name: "YAML => JSON",
  command: "sdt conv -a yaml -b json",
  input: true
}, {
  name: "YAML => TOML",
  command: "sdt conv -a yaml -b toml",
  input: true
}, {
  name: "YAML => QUERY",
  command: "sdt conv -a yaml -b query",
  input: true
}, {
  name: "YAML => CSV",
  command: "sdt conv -a yaml -b csv",
  input: true
}];
