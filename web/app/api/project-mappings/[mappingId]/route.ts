import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ mappingId: string }> };
export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to remove a workload mapping.");
  const { mappingId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(mappingId)) return errorResponse(400, "invalid_mapping_id", "The workload mapping identifier is invalid.");
  return backendResponse(await callBackend(`/v1/project-mappings/${encodeURIComponent(mappingId)}`, { method: "DELETE", token }));
}
