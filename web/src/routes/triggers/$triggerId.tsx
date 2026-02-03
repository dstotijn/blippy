import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { getAgent } from "@/lib/rpc/agent/agent-AgentService_connectquery";
import {
	deleteTrigger,
	getTrigger,
	updateTrigger,
} from "@/lib/rpc/trigger/trigger-TriggerService_connectquery";

export const Route = createFileRoute("/triggers/$triggerId")({
	component: TriggerDetail,
});

function TriggerDetail() {
	const { triggerId } = Route.useParams();
	const navigate = useNavigate();
	const { data: trigger, isLoading } = useQuery(getTrigger, { id: triggerId });
	const { data: agent } = useQuery(
		getAgent,
		{ id: trigger?.agentId ?? "" },
		{ enabled: !!trigger?.agentId },
	);
	const updateMutation = useMutation(updateTrigger);
	const deleteMutation = useMutation(deleteTrigger);

	const [name, setName] = useState("");
	const [prompt, setPrompt] = useState("");
	const [cronExpr, setCronExpr] = useState("");
	const [enabled, setEnabled] = useState(true);

	useEffect(() => {
		if (trigger) {
			setName(trigger.name);
			setPrompt(trigger.prompt);
			setCronExpr(trigger.cronExpr);
			setEnabled(trigger.enabled);
		}
	}, [trigger]);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			await updateMutation.mutateAsync({
				id: triggerId,
				name,
				prompt,
				cronExpr,
				enabled,
			});
			toast.success("Trigger updated");
		} catch {
			toast.error("Failed to update trigger");
		}
	};

	const handleDelete = async () => {
		if (!confirm("Are you sure you want to delete this trigger?")) return;
		try {
			await deleteMutation.mutateAsync({ id: triggerId });
			toast.success("Trigger deleted");
			navigate({ to: "/triggers" });
		} catch {
			toast.error("Failed to delete trigger");
		}
	};

	if (isLoading) {
		return (
			<div className="mx-auto max-w-2xl space-y-6">
				<Skeleton className="h-8 w-48" />
				<Card>
					<CardHeader>
						<Skeleton className="h-6 w-32" />
					</CardHeader>
					<CardContent className="space-y-4">
						<Skeleton className="h-10 w-full" />
						<Skeleton className="h-24 w-full" />
						<Skeleton className="h-10 w-full" />
					</CardContent>
				</Card>
			</div>
		);
	}

	if (!trigger) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Trigger not found
			</div>
		);
	}

	return (
		<div className="mx-auto max-w-2xl space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">{trigger.name}</h1>
					<p className="text-muted-foreground">Edit trigger settings</p>
				</div>
				<Button
					variant="destructive"
					size="icon"
					onClick={handleDelete}
					disabled={deleteMutation.isPending}
				>
					<Trash2 className="h-4 w-4" />
				</Button>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Trigger Settings</CardTitle>
					<CardDescription>Update the trigger configuration</CardDescription>
				</CardHeader>
				<CardContent>
					<form onSubmit={handleSubmit} className="space-y-6">
						<div className="space-y-2">
							<Label htmlFor="name">Name</Label>
							<Input
								id="name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								required
							/>
						</div>

						<div className="space-y-2">
							<Label>Agent</Label>
							<div className="flex h-10 w-full items-center rounded-md border border-input bg-muted px-3 py-2 text-sm">
								{agent?.name ?? "Loading..."}
							</div>
						</div>

						<div className="space-y-2">
							<Label htmlFor="prompt">Prompt</Label>
							<Textarea
								id="prompt"
								value={prompt}
								onChange={(e) => setPrompt(e.target.value)}
								rows={4}
								required
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="cronExpr">Cron Expression</Label>
							<Input
								id="cronExpr"
								value={cronExpr}
								onChange={(e) => setCronExpr(e.target.value)}
								placeholder="Leave empty for one-time triggers"
							/>
							<p className="text-xs text-muted-foreground">
								{trigger.nextRunAt
									? `Next run: ${timestampDate(trigger.nextRunAt).toLocaleString()}`
									: "No scheduled run"}
							</p>
						</div>

						<div className="flex items-center space-x-2">
							<Checkbox
								id="enabled"
								checked={enabled}
								onCheckedChange={(checked) => setEnabled(checked === true)}
							/>
							<Label htmlFor="enabled">Enabled</Label>
						</div>

						<Button type="submit" disabled={updateMutation.isPending}>
							{updateMutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</form>
				</CardContent>
			</Card>
		</div>
	);
}
