/** Newline-separated line numbers matching a source's line count. */
export function lineNumbers(source: string): string {
  const count = source.split("\n").length;
  return Array.from({ length: count }, (_, i) => i + 1).join("\n");
}
