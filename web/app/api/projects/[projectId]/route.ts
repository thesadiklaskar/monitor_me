import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ projectId: string }> };
export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { projectId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(projectId)) return errorResponse(400, "invalid_project_id", "The project identifier is invalid.");
  return backendResponse(await callBackend(`/v1/projects/${encodeURIComponent(projectId)}`, { method: "DELETE" }));
}
