import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { MessageSquare, Plus, Settings } from "lucide-react";
import { toast } from "sonner";
import { ConversationsTable } from "@/components/conversations-table";
import { EmptyState } from "@/components/empty-state";
import { PageContent } from "@/components/page-content";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { getAgent } from "@/lib/rpc/agent/agent-AgentService_connectquery";
import {
	createConversation,
	listConversations,
} from "@/lib/rpc/conversation/conversation-ConversationService_connectquery";

export const Route = createFileRoute("/agents/$agentId/")({
	component: ConversationsPage,
});

function ConversationsPage() {
	const { agentId } = Route.useParams();
	const navigate = useNavigate();
	const { data: agent } = useQuery(getAgent, { id: agentId });
	const { data, isLoading } = useQuery(listConversations, { agentId });
	const createConvMutation = useMutation(createConversation);

	const conversations = data?.conversations ?? [];

	const startNewConversation = async () => {
		try {
			const conv = await createConvMutation.mutateAsync({ agentId });
			toast.success("Conversation created");
			navigate({
				to: "/agents/$agentId/$conversationId",
				params: { agentId, conversationId: conv.id },
			});
		} catch {
			toast.error("Failed to create conversation");
		}
	};

	return (
		<PageContent className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">
						{agent?.name ?? "Agent"}
					</h1>
					{agent?.description && (
						<p className="text-muted-foreground">{agent.description}</p>
					)}
				</div>
				<div className="flex items-center gap-2">
					<Button variant="outline" size="icon" asChild>
						<Link to="/agents/$agentId/settings" params={{ agentId }}>
							<Settings className="h-4 w-4" />
							<span className="sr-only">Agent settings</span>
						</Link>
					</Button>
					<Button
						onClick={startNewConversation}
						disabled={createConvMutation.isPending}
					>
						<Plus className="h-4 w-4" />
						New Conversation
					</Button>
				</div>
			</div>

			{isLoading ? (
				<div className="space-y-2">
					<Skeleton className="h-10 w-full" />
					<Skeleton className="h-10 w-full" />
					<Skeleton className="h-10 w-full" />
				</div>
			) : conversations.length === 0 ? (
				<EmptyState
					icon={<MessageSquare />}
					title="No conversations yet"
					description="Start a new conversation to begin chatting"
					action={
						<Button
							onClick={startNewConversation}
							disabled={createConvMutation.isPending}
						>
							<Plus className="h-4 w-4" />
							Start Conversation
						</Button>
					}
				/>
			) : (
				<ConversationsTable conversations={conversations} agentId={agentId} />
			)}
		</PageContent>
	);
}
