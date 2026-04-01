import Fastify from "fastify";
import type { RuntimeState } from "./runtime.js";
import { redactAuthHeader, type Logger } from "./logger.js";
import { probeLitellmHealth, fetchLitellmModels } from "./litellm.js";
import {
  proxyChatCompletion,
  chatWithVirtualModelFallback,
} from "./chat.js";
import type { ChatCompletionBody } from "./types.js";

function bearerToken(authHeader: string | undefined): string | undefined {
  if (!authHeader) return undefined;
  const m = /^Bearer\s+(\S+)/i.exec(authHeader);
  return m?.[1];
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export async function buildServer(state: RuntimeState, log: Logger) {
  const app = Fastify({
    logger: false,
    trustProxy: true,
    bodyLimit: 25 * 1024 * 1024,
  });

  app.addHook("onResponse", (request, reply, done) => {
    log.info(
      {
        method: request.method,
        route: request.routeOptions?.url ?? request.url,
        path: request.url,
        statusCode: reply.statusCode,
        responseTimeMs: reply.elapsedTime,
        authorization: redactAuthHeader(request.headers.authorization),
      },
      "http response",
    );
    done();
  });

  app.get("/", async (_request, reply) => {
    state.syncGatewayConfig();
    const { semver, virtualModelId } = state.resolved;
    const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Claudia Gateway</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 40rem; margin: 3rem auto; padding: 0 1rem; line-height: 1.5; color: #1a1a1a; }
    h1 { font-size: 1.5rem; }
    .ok { color: #0d6832; font-weight: 600; }
    code { background: #f4f4f4; padding: 0.15em 0.4em; border-radius: 4px; font-size: 0.9em; }
    ul { padding-left: 1.2rem; }
    a { color: #0b57d0; }
  </style>
</head>
<body>
  <h1>Claudia Gateway</h1>
  <p class="ok">Up and operational.</p>
  <p>Version <code>${escapeHtml(semver)}</code> · Virtual model <code>${escapeHtml(virtualModelId)}</code></p>
  <p>OpenAI-compatible API under <code>/v1/</code> (e.g. chat and models). Use a gateway token for authenticated routes.</p>
  <ul>
    <li><a href="/health"><code>GET /health</code></a> — JSON readiness (LiteLLM probe)</li>
  </ul>
</body>
</html>`;
    return reply.type("text/html; charset=utf-8").send(html);
  });

  app.get("/health", async (_request, reply) => {
    state.syncGatewayConfig();
    const { healthLitellmUrl, healthTimeoutMs } = state.resolved;
    const litellmKey = state.litellmApiKey();
    const litellm = await probeLitellmHealth(
      healthLitellmUrl,
      healthTimeoutMs,
      log,
      litellmKey,
    );
    const checks = {
      litellm: {
        ok: litellm.ok,
        status: litellm.status,
        ...(litellm.detail ? { detail: litellm.detail } : {}),
      },
    };
    if (!litellm.ok) {
      return reply.code(503).send({
        degraded: true,
        status: "degraded",
        checks,
      });
    }
    return reply.send({ status: "ok", checks });
  });

  app.get("/v1/models", async (request, reply) => {
    state.syncGatewayConfig();
    const token = bearerToken(request.headers.authorization);
    const session = token ? state.tokens.validate(token) : undefined;
    if (!token || !session) {
      return reply.code(401).send({
        error: { message: "Unauthorized", type: "invalid_api_key" },
      });
    }

    const apiKey = state.litellmApiKey();
    if (!apiKey) {
      return reply.code(503).send({
        error: {
          message: `Missing ${state.resolved.litellmApiKeyEnv} for upstream LiteLLM`,
          type: "gateway_config",
        },
      });
    }

    const { litellmBaseUrl, healthTimeoutMs, virtualModelId } =
      state.resolved;
    const upstream = await fetchLitellmModels(
      litellmBaseUrl,
      apiKey,
      healthTimeoutMs,
      log,
    );
    if (!upstream.ok || !upstream.json) {
      return reply.code(502).send({
        error: {
          message: "Failed to list models from LiteLLM",
          type: "gateway_upstream",
          status: upstream.status,
        },
      });
    }

    const virtual = {
      id: virtualModelId,
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: "claudia-gateway",
    };
    const data = [
      virtual,
      ...((upstream.json.data as Record<string, unknown>[]) ?? []),
    ];
    return reply.send({ object: "list", data });
  });

  app.post("/v1/chat/completions", async (request, reply) => {
    state.syncGatewayConfig();
    const token = bearerToken(request.headers.authorization);
    const session = token ? state.tokens.validate(token) : undefined;
    if (!token || !session) {
      return reply.code(401).send({
        error: { message: "Unauthorized", type: "invalid_api_key" },
      });
    }

    const apiKey = state.litellmApiKey();
    if (!apiKey) {
      return reply.code(503).send({
        error: {
          message: `Missing ${state.resolved.litellmApiKeyEnv} for upstream LiteLLM`,
          type: "gateway_config",
        },
      });
    }

    const body = request.body as ChatCompletionBody;
    if (!body || typeof body !== "object") {
      return reply.code(400).send({
        error: { message: "Expected JSON body", type: "invalid_request" },
      });
    }

    const stream = Boolean(body.stream);
    const {
      virtualModelId,
      fallbackChain,
      litellmBaseUrl,
      chatTimeoutMs,
    } = state.resolved;

    log.info(
      {
        clientModel: body.model,
        stream,
        tenant: session.tenantId,
      },
      "chat completion request",
    );

    if (body.model === virtualModelId) {
      const pick = state.routing.pickInitialModel(
        body,
        fallbackChain,
        virtualModelId,
      );
      if (!pick.model) {
        return reply.code(503).send({
          error: {
            message:
              "Could not resolve an initial LiteLLM model for the virtual Claudia model (check routing policy and fallback chain).",
            type: "gateway_config",
          },
        });
      }
      await chatWithVirtualModelFallback({
        body,
        initialUpstreamModel: pick.model,
        fallbackChain,
        litellmBaseUrl,
        litellmApiKey: apiKey,
        stream,
        timeoutMs: chatTimeoutMs,
        log,
        reply,
      });
      return;
    }

    if (!body.model) {
      return reply.code(400).send({
        error: { message: "Missing model", type: "invalid_request" },
      });
    }

    const result = await proxyChatCompletion({
      body,
      litellmBaseUrl,
      litellmApiKey: apiKey,
      upstreamModel: body.model,
      stream,
      timeoutMs: chatTimeoutMs,
      log,
      reply,
    });

    if (result.kind === "stream") return;
    if (result.kind === "json") {
      return reply.code(result.status).send(result.body);
    }
    return reply.code(result.status).send({
      error: { message: result.message, type: "gateway_upstream" },
    });
  });

  return app;
}
