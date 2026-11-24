import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ mappingId: string }> };
export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { mappingId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(mappingId)) return errorResponse(400, "invalid_mapping_id", "The workload mapping identifier is invalid.");
  return backendResponse(await callBackend(`/v1/project-mappings/${encodeURIComponent(mappingId)}`, { method: "DELETE" }));
}
