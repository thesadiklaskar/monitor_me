import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  sessionToken,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

type RouteContext = { params: Promise<{ serverId: string }> };

export async function GET(request: NextRequest, context: RouteContext) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view server metrics.");

  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) {
    return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  }

  const allowed = ["metric", "from", "to", "step", "limit", "project_id"];
  const query = new URLSearchParams();
  for (const key of allowed) {
    const value = request.nextUrl.searchParams.get(key);
    if (value !== null) query.set(key, value);
  }
  return backendResponse(
    await callBackend(`/v1/servers/${encodeURIComponent(serverId)}/metrics?${query.toString()}`, { token }),
  );
}
