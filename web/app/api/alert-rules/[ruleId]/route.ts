import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ ruleId: string }> };
export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to delete an alert rule.");
  const { ruleId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(ruleId)) return errorResponse(400, "invalid_rule_id", "The alert rule identifier is invalid.");
  return backendResponse(await callBackend("/v1/alert-rules/" + encodeURIComponent(ruleId), { method: "DELETE", token }));
}
