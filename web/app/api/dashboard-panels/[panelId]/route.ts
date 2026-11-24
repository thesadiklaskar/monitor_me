import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation, sessionToken } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ panelId: string }> };

export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to remove a panel.");
  const { panelId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(panelId)) return errorResponse(400, "invalid_panel_id", "The panel identifier is invalid.");
  return backendResponse(await callBackend("/v1/dashboard-panels/" + encodeURIComponent(panelId), { method: "DELETE", token }));
}
