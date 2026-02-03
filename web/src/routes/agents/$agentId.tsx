import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/agents/$agentId")({
	component: AgentLayout,
});

function AgentLayout() {
	return <Outlet />;
}
