import { useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Bot, MessageSquare, Plus, Settings } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageContent } from "@/components/page-content";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { listAgents } from "@/lib/rpc/agent/agent-AgentService_connectquery";

export const Route = createFileRoute("/")({
	component: Index,
});

function Index() {
	const { data, isLoading, error } = useQuery(listAgents);

	if (error) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Error: {error.message}
			</div>
		);
	}

	const agents = data?.agents ?? [];

	return (
		<PageContent className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">Agents</h1>
					<p className="text-muted-foreground">
						Manage your AI agents and start conversations
					</p>
				</div>
				<Button asChild>
					<Link to="/agents/new">
						<Plus className="h-4 w-4" />
						New Agent
					</Link>
				</Button>
			</div>

			{isLoading ? (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{[...Array(3)].map((_, i) => (
						// biome-ignore lint/suspicious/noArrayIndexKey: Static skeleton placeholders never reorder
						<Card key={`skeleton-${i}`}>
							<CardHeader>
								<Skeleton className="h-5 w-32" />
								<Skeleton className="h-4 w-48" />
							</CardHeader>
						</Card>
					))}
				</div>
			) : agents.length === 0 ? (
				<EmptyState
					icon={<Bot />}
					title="No agents yet"
					description="Create your first AI agent to get started"
					action={
						<Button asChild>
							<Link to="/agents/new">
								<Plus className="h-4 w-4" />
								Create Agent
							</Link>
						</Button>
					}
				/>
			) : (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{agents.map((agent) => (
						<Card
							key={agent.id}
							className="group transition-colors hover:border-foreground/20"
						>
							<CardHeader>
								<div className="flex items-center justify-between">
									<div className="space-y-1">
										<CardTitle className="text-base">
											<Link
												to="/agents/$agentId"
												params={{ agentId: agent.id }}
												className="hover:underline"
											>
												{agent.name}
											</Link>
										</CardTitle>
										{agent.description && (
											<CardDescription className="line-clamp-2">
												{agent.description}
											</CardDescription>
										)}
									</div>
									<div className="flex gap-1">
										<Button variant="ghost" size="icon" asChild>
											<Link
												to="/agents/$agentId"
												params={{ agentId: agent.id }}
											>
												<MessageSquare className="h-4 w-4" />
												<span className="sr-only">Chat with {agent.name}</span>
											</Link>
										</Button>
										<Button variant="ghost" size="icon" asChild>
											<Link
												to="/agents/$agentId/settings"
												params={{ agentId: agent.id }}
											>
												<Settings className="h-4 w-4" />
												<span className="sr-only">
													Settings for {agent.name}
												</span>
											</Link>
										</Button>
									</div>
								</div>
							</CardHeader>
						</Card>
					))}
				</div>
			)}
		</PageContent>
	);
}
