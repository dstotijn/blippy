import { useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Bell, Plus } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { listNotificationChannels } from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";

export const Route = createFileRoute("/notifications/")({
	component: NotificationsIndex,
});

function NotificationsIndex() {
	const { data, isLoading, error } = useQuery(listNotificationChannels, {});

	if (error) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Error: {error.message}
			</div>
		);
	}

	const channels = data?.channels ?? [];

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">
						Notification Channels
					</h1>
					<p className="text-muted-foreground">
						Configure where agents can send notifications
					</p>
				</div>
				<Button asChild>
					<Link to="/notifications/new">
						<Plus className="h-4 w-4" />
						New Channel
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
			) : channels.length === 0 ? (
				<EmptyState
					icon={<Bell />}
					title="No notification channels"
					description="Add a channel to let agents send notifications"
					action={
						<Button asChild>
							<Link to="/notifications/new">
								<Plus className="h-4 w-4" />
								Add Channel
							</Link>
						</Button>
					}
				/>
			) : (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{channels.map((channel) => (
						<Card
							key={channel.id}
							className="group transition-colors hover:border-foreground/20"
						>
							<CardHeader>
								<div className="space-y-1">
									<CardTitle className="text-base">
										<Link
											to="/notifications/$channelId"
											params={{ channelId: channel.id }}
											className="hover:underline"
										>
											{channel.name}
										</Link>
									</CardTitle>
									<CardDescription>Type: {channel.type}</CardDescription>
								</div>
							</CardHeader>
						</Card>
					))}
				</div>
			)}
		</div>
	);
}
