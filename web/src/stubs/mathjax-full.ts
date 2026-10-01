// Browser stub for `mathjax-full`.
//
// marp-core statically requires mathjax-full at module load even when it is
// constructed with `math: 'katex'`, so the whole MathJax library (several MB)
// would otherwise land in the lazy slide chunk. The viewer never selects the
// MathJax renderer, so this stub satisfies the requires without shipping it.
//
// If a deck ever needs the MathJax renderer, the alias must be removed: this
// module does not implement it.

function unsupported(): never {
  throw new Error("mathjax-full is stubbed out; the viewer renders math with KaTeX");
}

export const liteAdaptor = unsupported;
export const RegisterHTMLHandler = unsupported;
export const TeX = unsupported;
export const SVG = unsupported;
export const AllPackages = unsupported;
const mathjax = { document: unsupported };
export default mathjax;
