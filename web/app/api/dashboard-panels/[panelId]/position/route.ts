import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ panelId: string }> };

export async function PATCH(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { panelId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(panelId)) return errorResponse(400, "invalid_panel_id", "The panel identifier is invalid.");
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return errorResponse(400, "invalid_json", "The request body is not valid JSON.");
  }
  if (!body || typeof body !== "object" || !Number.isInteger((body as { position?: unknown }).position)) {
    return errorResponse(400, "invalid_position", "A numeric panel position is required.");
  }
  return backendResponse(await callBackend("/v1/dashboard-panels/" + encodeURIComponent(panelId) + "/position", {
    method: "PATCH",
    body: JSON.stringify({ position: (body as { position: number }).position }),
  }));
}
