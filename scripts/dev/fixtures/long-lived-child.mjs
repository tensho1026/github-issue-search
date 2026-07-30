import { appendFileSync, writeFileSync } from "node:fs";
import process from "node:process";

const [pidFile, stoppedFile] = process.argv.slice(2);
if (!pidFile || !stoppedFile) {
  throw new Error("expected PID and stopped marker paths");
}

writeFileSync(pidFile, String(process.pid));

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.once(signal, () => {
    appendFileSync(stoppedFile, `${process.pid}:${signal}\n`);
    process.exit(0);
  });
}

setInterval(() => {}, 1_000);
