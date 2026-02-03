import { useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Clock, Plus } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { listTriggers } from "@/lib/rpc/trigger/trigger-TriggerService_connectquery";

export const Route = createFileRoute("/triggers/")({
	component: TriggersIndex,
});

function TriggersIndex() {
	const { data, isLoading, error } = useQuery(listTriggers, {});

	if (error) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Error: {error.message}
			</div>
		);
	}

	const triggers = data?.triggers ?? [];

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">Triggers</h1>
					<p className="text-muted-foreground">
						Schedule agents to run automatically
					</p>
				</div>
				<Button asChild>
					<Link to="/triggers/new">
						<Plus className="h-4 w-4" />
						New Trigger
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
			) : triggers.length === 0 ? (
				<EmptyState
					icon={<Clock />}
					title="No triggers yet"
					description="Create a trigger to run agents on a schedule"
					action={
						<Button asChild>
							<Link to="/triggers/new">
								<Plus className="h-4 w-4" />
								Create Trigger
							</Link>
						</Button>
					}
				/>
			) : (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{triggers.map((trigger) => (
						<Card
							key={trigger.id}
							className="group transition-colors hover:border-foreground/20"
						>
							<CardHeader>
								<div className="flex items-start justify-between">
									<div className="space-y-1">
										<CardTitle className="text-base">
											<Link
												to="/triggers/$triggerId"
												params={{ triggerId: trigger.id }}
												className="hover:underline"
											>
												{trigger.name}
											</Link>
										</CardTitle>
										<CardDescription className="line-clamp-2">
											{trigger.cronExpr
												? `Cron: ${trigger.cronExpr}`
												: "One-time trigger"}
										</CardDescription>
										<div className="flex items-center gap-2 text-xs text-muted-foreground">
											<span
												className={`inline-block h-2 w-2 rounded-full ${trigger.enabled ? "bg-green-500" : "bg-gray-400"}`}
											/>
											{trigger.enabled ? "Enabled" : "Disabled"}
										</div>
									</div>
								</div>
							</CardHeader>
						</Card>
					))}
				</div>
			)}
		</div>
	);
}
