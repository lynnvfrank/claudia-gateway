import { pipeline } from "node:stream/promises";
import { Readable } from "node:stream";
import type { FastifyReply } from "fastify";
import type { Logger } from "./logger.js";
import type { ChatCompletionBody } from "./types.js";
import { startingFallbackIndex } from "./routing.js";

const RETRY_STATUSES = new Set([429, 500, 502, 503, 504]);

export async function proxyChatCompletion(params: {
  body: ChatCompletionBody;
  litellmBaseUrl: string;
  litellmApiKey: string;
  upstreamModel: string;
  stream: boolean;
  timeoutMs: number;
  log: Logger;
  reply: FastifyReply;
}): Promise<
  | { kind: "stream" }
  | { kind: "json"; status: number; body: unknown }
  | { kind: "error"; status: number; message: string }
> {
  const {
    body,
    litellmBaseUrl,
    litellmApiKey,
    upstreamModel,
    stream,
    timeoutMs,
    log,
    reply,
  } = params;

  const url = `${litellmBaseUrl.replace(/\/+$/, "")}/v1/chat/completions`;
  const payload = { ...body, model: upstreamModel, stream };

  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);

  log.debug(
    {
      upstreamModel,
      stream,
      target: url,
    },
    "litellm chat relay",
  );

  try {
    const res = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${litellmApiKey}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
      signal: ac.signal,
    });

    log.info(
      {
        route: "POST /v1/chat/completions (upstream)",
        target: url,
        status: res.status,
        upstreamModel,
        stream,
      },
      "litellm chat response",
    );

    if (!res.ok && !stream) {
      let errBody: unknown;
      try {
        errBody = await res.json();
      } catch {
        errBody = { error: await res.text() };
      }
      return { kind: "json", status: res.status, body: errBody };
    }

    if (!res.ok && stream) {
      return {
        kind: "error",
        status: res.status,
        message: "upstream error on streaming request",
      };
    }

    if (stream && res.body) {
      const headersOut: Record<string, string> = {
        "Content-Type":
          res.headers.get("content-type") ?? "text/event-stream; charset=utf-8",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      };
      const xRequestId = res.headers.get("x-request-id");
      if (xRequestId) headersOut["x-request-id"] = xRequestId;

      reply.hijack();
      reply.raw.writeHead(200, headersOut);
      const nodeStream = Readable.fromWeb(
        res.body as import("stream/web").ReadableStream,
      );
      await pipeline(nodeStream, reply.raw);
      return { kind: "stream" };
    }

    const json = await res.json();
    return { kind: "json", status: res.status, body: json };
  } catch (e) {
    log.info({ err: e, target: url, upstreamModel, stream }, "litellm chat fetch failed");
    return {
      kind: "error",
      status: 503,
      message: e instanceof Error ? e.message : String(e),
    };
  } finally {
    clearTimeout(t);
  }
}

export async function chatWithVirtualModelFallback(params: {
  body: ChatCompletionBody;
  initialUpstreamModel: string;
  fallbackChain: string[];
  litellmBaseUrl: string;
  litellmApiKey: string;
  stream: boolean;
  timeoutMs: number;
  log: Logger;
  reply: FastifyReply;
}): Promise<void> {
  const {
    body,
    initialUpstreamModel,
    fallbackChain,
    litellmBaseUrl,
    litellmApiKey,
    stream,
    timeoutMs,
    log,
    reply,
  } = params;

  const start = startingFallbackIndex(initialUpstreamModel, fallbackChain);
  const chain =
    fallbackChain.length > 0
      ? fallbackChain.slice(start)
      : initialUpstreamModel
        ? [initialUpstreamModel]
        : [];

  if (!chain.length) {
    reply.code(503).send({
      error: {
        message:
          "No LiteLLM models configured (routing.fallback_chain empty and no initial model).",
        type: "gateway_config",
      },
    });
    return;
  }

  let lastNonRetry: { status: number; body: unknown } | null = null;

  for (let i = 0; i < chain.length; i++) {
    const upstreamModel = chain[i];
    log.debug(
      { attempt: i + 1, upstreamModel, chainLen: chain.length },
      "virtual model fallback attempt",
    );

    const result = await proxyChatCompletion({
      body,
      litellmBaseUrl,
      litellmApiKey,
      upstreamModel,
      stream,
      timeoutMs,
      log,
      reply,
    });

    if (result.kind === "stream") return;

    if (result.kind === "error") {
      if (RETRY_STATUSES.has(result.status) && i < chain.length - 1) continue;
      reply.code(result.status).send({
        error: { message: result.message, type: "gateway_upstream" },
      });
      return;
    }

    if (result.kind === "json") {
      if (RETRY_STATUSES.has(result.status) && i < chain.length - 1) {
        log.info(
          { upstreamModel, status: result.status, willRetry: true },
          "retrying next fallback model",
        );
        continue;
      }
      if (!RETRY_STATUSES.has(result.status)) {
        reply.code(result.status).send(result.body);
        return;
      }
      lastNonRetry = { status: result.status, body: result.body };
    }
  }

  if (lastNonRetry) {
    reply.code(lastNonRetry.status).send(lastNonRetry.body);
    return;
  }

  reply.code(503).send({
    error: {
      message: "Exhausted fallback chain without a successful completion.",
      type: "gateway_exhausted",
    },
  });
}
