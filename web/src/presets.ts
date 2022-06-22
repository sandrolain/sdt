
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
  name: "Help",
  command: "sdt help",
  input: false
}, {
  name: "Random bytes Base64",
  command: "sdt : bytes : b64",
  input: false
}, {
  name: "Random bytes Hexadecimal",
  command: "sdt : bytes : hex",
  input: false
}, {
  name: "Base 32 Encode",
  command: "sdt b32",
  input: true
}, {
  name: "Base 32 Decode",
  command: "sdt b32 dec",
  input: true
}, {
  name: "Base 64 Encode",
  command: "sdt b64",
  input: true
}, {
  name: "Base 64 Decode",
  command: "sdt b64 dec",
  input: true
}, {
  name: "Base 64 URL-Encode",
  command: "sdt b64url",
  input: true
}, {
  name: "Base 64 URL-Decode",
  command: "sdt : url decode : b64url decode",
  input: true
}, {
  name: "URL encode",
  command: "sdt url encode",
  input: true
}, {
  name: "URL decode",
  command: "sdt url decode",
  input: true
}, {
  name: "HTML encode",
  command: "sdt html encode",
  input: true
}, {
  name: "HTML decode",
  command: "sdt html decode",
  input: true
}, {
  name: "URL Encode",
  command: "sdt url enc",
  input: true
}, {
  name: "URL Form-Encode",
  command: "sdt url encform",
  input: true
}, {
  name: "URL Decode",
  command: "sdt url dec",
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
}, {
  name: "RSA Key pair",
  command: "sdt keypair",
  input: false
}, {
  name: "String: UPPER CASE",
  command: "sdt string uppercase",
  input: true
}, {
  name: "String: lower case",
  command: "sdt string lowercase",
  input: true
}, {
  name: "String: Title Case",
  command: "sdt string titlecase",
  input: true
}, {
  name: "String: Header-Case",
  command: "sdt : string titlecase : string replacespace -r=-",
  input: true
}, {
  name: "String: param-case",
  command: "sdt : string lowercase : string replacespace -r=-",
  input: true
}, {
  name: "String: snake_case",
  command: "sdt : string lowercase : string replacespace -r=_",
  input: true
}];
