import pino from "pino";

export function createLogger(level: string) {
  return pino({
    level: level || "info",
    formatters: {
      level(label) {
        return { level: label };
      },
    },
    timestamp: pino.stdTimeFunctions.isoTime,
  });
}

export type Logger = ReturnType<typeof createLogger>;

export function redactAuthHeader(h: string | undefined): string {
  if (!h) return "";
  const m = /^Bearer\s+(\S+)/i.exec(h);
  if (!m) return "[non-bearer]";
  const t = m[1];
  if (t.length <= 8) return "Bearer ***";
  return `Bearer ${t.slice(0, 4)}…${t.slice(-4)}`;
}
