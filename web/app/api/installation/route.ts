import { NextRequest } from "next/server";
import { backendResponse, callBackend, sessionToken } from "@/lib/server-api";

export const dynamic = "force-dynamic";

// Initialize and identify the local installation without creating browser
// accounts, login sessions, or an organization-selection workflow.
export async function GET(request: NextRequest) {
  return backendResponse(
    await callBackend("/v1/installation", { token: sessionToken(request) }),
  );
}
