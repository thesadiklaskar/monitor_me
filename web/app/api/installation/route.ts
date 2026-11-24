import { backendResponse, callBackend } from "@/lib/server-api";

export const dynamic = "force-dynamic";

// Initialize and identify the local installation without creating browser
// accounts, login sessions, or an organization-selection workflow.
export async function GET() {
  return backendResponse(
    await callBackend("/v1/installation"),
  );
}
