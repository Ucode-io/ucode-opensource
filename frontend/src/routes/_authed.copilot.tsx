import { createFileRoute } from "@tanstack/react-router";
import { CopilotPage } from "@/features/copilot";

/** Помощник отдельным экраном — вход из сайдбара. Панель справа остаётся. */
export const Route = createFileRoute("/_authed/copilot")({ component: CopilotPage });
