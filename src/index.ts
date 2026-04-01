import { createLogger } from "./logger.js";
import { resolveGatewayConfigPath } from "./config.js";
import { RuntimeState } from "./runtime.js";
import { buildServer } from "./server.js";

async function main() {
  const configPath = resolveGatewayConfigPath();
  const log = createLogger(process.env.LOG_LEVEL?.trim() ?? "info");
  let state: RuntimeState;
  try {
    state = new RuntimeState(log);
  } catch (e) {
    log.fatal({ err: e, path: configPath }, "cannot load gateway.yaml");
    process.exit(1);
  }

  const app = await buildServer(state, log);
  const { listenHost, listenPort } = state.resolved;

  await app.listen({ host: listenHost, port: listenPort });
  log.info(
    { host: listenHost, port: listenPort, configPath },
    "claudia gateway listening",
  );
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
