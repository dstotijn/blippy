import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { listAgents } from "@/lib/rpc/agent/agent-AgentService_connectquery";
import { createTrigger } from "@/lib/rpc/trigger/trigger-TriggerService_connectquery";

export const Route = createFileRoute("/triggers/new")({
	component: NewTrigger,
});

function NewTrigger() {
	const navigate = useNavigate();
	const { data: agentsData } = useQuery(listAgents);
	const mutation = useMutation(createTrigger);

	const [name, setName] = useState("");
	const [agentId, setAgentId] = useState("");
	const [prompt, setPrompt] = useState("");
	const [scheduleType, setScheduleType] = useState<"cron" | "delay">("cron");
	const [cronExpr, setCronExpr] = useState("");
	const [delay, setDelay] = useState("");

	const agents = agentsData?.agents ?? [];

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			const trigger = await mutation.mutateAsync({
				name,
				agentId,
				prompt,
				cronExpr: scheduleType === "cron" ? cronExpr : "",
				delay: scheduleType === "delay" ? delay : "",
			});
			toast.success("Trigger created");
			navigate({
				to: "/triggers/$triggerId",
				params: { triggerId: trigger.id },
			});
		} catch {
			toast.error("Failed to create trigger");
		}
	};

	return (
		<div className="mx-auto max-w-2xl space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">New Trigger</h1>
				<p className="text-muted-foreground">
					Schedule an agent to run automatically
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Trigger Details</CardTitle>
					<CardDescription>
						Configure when and how the agent should run
					</CardDescription>
				</CardHeader>
				<CardContent>
					<form onSubmit={handleSubmit} className="space-y-6">
						<div className="space-y-2">
							<Label htmlFor="name">Name</Label>
							<Input
								id="name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="e.g., Daily Summary"
								required
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="agentId">Agent</Label>
							<Select value={agentId} onValueChange={setAgentId}>
								<SelectTrigger id="agentId">
									<SelectValue placeholder="Select an agent..." />
								</SelectTrigger>
								<SelectContent>
									{agents.map((agent) => (
										<SelectItem key={agent.id} value={agent.id}>
											{agent.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>

						<div className="space-y-2">
							<Label htmlFor="prompt">Prompt</Label>
							<Textarea
								id="prompt"
								value={prompt}
								onChange={(e) => setPrompt(e.target.value)}
								placeholder="What should the agent do?"
								rows={4}
								required
							/>
						</div>

						<div className="space-y-4">
							<Label>Schedule Type</Label>
							<div className="flex gap-4">
								<label className="flex items-center gap-2">
									<input
										type="radio"
										name="scheduleType"
										checked={scheduleType === "cron"}
										onChange={() => setScheduleType("cron")}
										className="h-4 w-4"
									/>
									<span className="text-sm">Recurring (Cron)</span>
								</label>
								<label className="flex items-center gap-2">
									<input
										type="radio"
										name="scheduleType"
										checked={scheduleType === "delay"}
										onChange={() => setScheduleType("delay")}
										className="h-4 w-4"
									/>
									<span className="text-sm">One-time (Delay)</span>
								</label>
							</div>

							{scheduleType === "cron" ? (
								<div className="space-y-2">
									<Label htmlFor="cronExpr">Cron Expression</Label>
									<Input
										id="cronExpr"
										value={cronExpr}
										onChange={(e) => setCronExpr(e.target.value)}
										placeholder="e.g., 0 9 * * * (daily at 9am)"
										required={scheduleType === "cron"}
									/>
									<p className="text-xs text-muted-foreground">
										Format: minute hour day-of-month month day-of-week
									</p>
								</div>
							) : (
								<div className="space-y-2">
									<Label htmlFor="delay">Delay</Label>
									<Input
										id="delay"
										value={delay}
										onChange={(e) => setDelay(e.target.value)}
										placeholder="e.g., 5m, 1h, 24h"
										required={scheduleType === "delay"}
									/>
									<p className="text-xs text-muted-foreground">
										Run once after this delay
									</p>
								</div>
							)}
						</div>

						<div className="flex gap-3">
							<Button type="submit" disabled={mutation.isPending}>
								{mutation.isPending ? "Creating..." : "Create Trigger"}
							</Button>
							<Button variant="outline" asChild>
								<Link to="/triggers">Cancel</Link>
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</div>
	);
}
