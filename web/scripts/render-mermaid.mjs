import { render } from "pele";

let input = "";
for await (const chunk of process.stdin) input += chunk;

const request = JSON.parse(input);
const results = request.diagrams.map(({ source }) => {
  try {
    return { svg: render(source).svg };
  } catch (error) {
    return { error: error instanceof Error ? error.message : String(error) };
  }
});

process.stdout.write(JSON.stringify({ results }));
