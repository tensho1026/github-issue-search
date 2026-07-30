import { spawn, spawnSync } from "node:child_process";
import process from "node:process";

const defaultStopTimeoutMs = 10_000;

export class NativeProcessGroup {
  #children = [];
  #resolveUnexpectedExit;
  #stopping = false;
  #unexpectedExit;

  constructor({ output = process.stderr } = {}) {
    this.output = output;
    this.#unexpectedExit = new Promise((resolve) => {
      this.#resolveUnexpectedExit = resolve;
    });
  }

  start({ name, command, args = [], cwd, env = process.env }) {
    if (this.#stopping) {
      throw new Error(
        "cannot start a service while the process group is stopping",
      );
    }

    const child = spawn(command, args, {
      cwd,
      detached: process.platform !== "win32",
      env,
      stdio: "inherit",
    });
    const service = { child, name };
    this.#children.push(service);

    child.once("error", (error) => {
      if (!this.#stopping) {
        this.#resolveUnexpectedExit(
          new Error(`${name} failed to start: ${error.message}`, {
            cause: error,
          }),
        );
      }
    });
    child.once("exit", (code, signal) => {
      if (!this.#stopping) {
        this.#resolveUnexpectedExit(
          new Error(
            `${name} exited unexpectedly (code=${String(code)}, signal=${String(signal)})`,
          ),
        );
      }
    });

    return child;
  }

  async waitForUnexpectedExit() {
    throw await this.#unexpectedExit;
  }

  async stop({ timeoutMs = defaultStopTimeoutMs } = {}) {
    if (this.#stopping) {
      return;
    }
    this.#stopping = true;

    const running = this.#children.filter(
      ({ child }) => child.exitCode === null && child.signalCode === null,
    );
    for (const service of running.toReversed()) {
      terminateTree(service.child, "SIGTERM");
    }

    const graceful = await Promise.all(
      running.map(({ child }) => waitForExit(child, timeoutMs)),
    );
    for (const [index, exited] of graceful.entries()) {
      if (exited) {
        continue;
      }
      const service = running[index];
      this.output.write(
        `[native-stack] ${service.name} exceeded the ${timeoutMs}ms shutdown deadline; forcing termination.\n`,
      );
      terminateTree(service.child, "SIGKILL");
    }
    await Promise.all(
      running.map(({ child }) =>
        waitForExit(child, Math.min(timeoutMs, 2_000)),
      ),
    );
  }
}

function terminateTree(child, signal) {
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }

  try {
    if (process.platform === "win32") {
      const args = ["/pid", String(child.pid), "/t"];
      if (signal === "SIGKILL") {
        args.push("/f");
      }
      spawnSync("taskkill", args, { stdio: "ignore" });
      return;
    }
    process.kill(-child.pid, signal);
  } catch (error) {
    child.kill(signal);
  }
}

function waitForExit(child, timeoutMs) {
  if (child.exitCode !== null || child.signalCode !== null) {
    return Promise.resolve(true);
  }

  return new Promise((resolve) => {
    const timeout = setTimeout(() => {
      child.off("exit", onExit);
      resolve(false);
    }, timeoutMs);
    timeout.unref();

    function onExit() {
      clearTimeout(timeout);
      resolve(true);
    }
    child.once("exit", onExit);
  });
}
