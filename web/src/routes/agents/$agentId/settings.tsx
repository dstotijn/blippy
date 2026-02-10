import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute } from "@tanstack/react-router";
import { Check, ChevronsUpDown } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { PageContent } from "@/components/page-content";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Command,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
} from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
	getAgent,
	listModels,
	updateAgent,
} from "@/lib/rpc/agent/agent-AgentService_connectquery";
import { listNotificationChannels } from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/agents/$agentId/settings")({
	component: AgentPage,
});

function AgentPage() {
	const { agentId } = Route.useParams();
	const { data: agent, isLoading, error } = useQuery(getAgent, { id: agentId });
	const { data: channelsData } = useQuery(listNotificationChannels, {});
	const { data: modelsData } = useQuery(listModels, {});
	const updateMutation = useMutation(updateAgent);

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [systemPrompt, setSystemPrompt] = useState("");
	const [enabledTools, setEnabledTools] = useState<string[]>([]);
	const [enabledNotificationChannels, setEnabledNotificationChannels] =
		useState<string[]>([]);
	const [model, setModel] = useState("");
	const [modelOpen, setModelOpen] = useState(false);

	useEffect(() => {
		if (agent) {
			setName(agent.name);
			setDescription(agent.description);
			setSystemPrompt(agent.systemPrompt);
			setEnabledTools(agent.enabledTools || []);
			setEnabledNotificationChannels(agent.enabledNotificationChannels || []);
			setModel(agent.model);
		}
	}, [agent]);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			await updateMutation.mutateAsync({
				id: agentId,
				name,
				description,
				systemPrompt,
				enabledTools,
				enabledNotificationChannels,
				model,
			});
			toast.success("Agent updated");
		} catch {
			toast.error("Failed to update agent");
		}
	};

	const toggleTool = (toolName: string) => {
		setEnabledTools((prev) =>
			prev.includes(toolName)
				? prev.filter((t) => t !== toolName)
				: [...prev, toolName],
		);
	};

	const toggleNotificationChannel = (channelId: string) => {
		setEnabledNotificationChannels((prev) =>
			prev.includes(channelId)
				? prev.filter((id) => id !== channelId)
				: [...prev, channelId],
		);
	};

	if (error) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Error: {error.message}
			</div>
		);
	}

	if (isLoading) {
		return (
			<PageContent className="mx-auto max-w-2xl space-y-6">
				<div className="space-y-2">
					<Skeleton className="h-8 w-48" />
					<Skeleton className="h-4 w-64" />
				</div>
				<Card>
					<CardHeader>
						<Skeleton className="h-6 w-32" />
						<Skeleton className="h-4 w-48" />
					</CardHeader>
					<CardContent className="space-y-6">
						<Skeleton className="h-9 w-full" />
						<Skeleton className="h-9 w-full" />
						<Skeleton className="h-32 w-full" />
					</CardContent>
				</Card>
			</PageContent>
		);
	}

	if (!agent) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Agent not found
			</div>
		);
	}

	return (
		<PageContent className="mx-auto max-w-2xl space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">{agent.name}</h1>
				<p className="text-muted-foreground">Agent settings</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Agent Details</CardTitle>
					<CardDescription>
						Update your agent's configuration and behavior
					</CardDescription>
				</CardHeader>
				<CardContent>
					<form onSubmit={handleSubmit} className="space-y-6">
						<div className="space-y-2">
							<Label htmlFor="name">Name</Label>
							<Input
								id="name"
								type="text"
								value={name}
								onChange={(e) => setName(e.target.value)}
								required
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="description">Description</Label>
							<Input
								id="description"
								type="text"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="systemPrompt">System Prompt</Label>
							<Textarea
								id="systemPrompt"
								value={systemPrompt}
								onChange={(e) => setSystemPrompt(e.target.value)}
								rows={8}
							/>
							<p className="text-xs text-muted-foreground">
								Define the agent's personality, capabilities, and constraints
							</p>
						</div>

						<div className="space-y-2">
							<Label>Model</Label>
							<Popover open={modelOpen} onOpenChange={setModelOpen}>
								<PopoverTrigger asChild>
									<Button
										variant="outline"
										role="combobox"
										aria-expanded={modelOpen}
										className="w-full justify-between font-normal"
									>
										{model
											? (modelsData?.models.find((m) => m.id === model)?.name ??
												model)
											: "Default"}
										<ChevronsUpDown className="opacity-50" />
									</Button>
								</PopoverTrigger>
								<PopoverContent
									className="w-[--radix-popover-trigger-width] p-0"
									align="start"
								>
									<Command>
										<CommandInput placeholder="Search models..." />
										<CommandList>
											<CommandEmpty>No model found.</CommandEmpty>
											<CommandGroup>
												<CommandItem
													value="default"
													onSelect={() => {
														setModel("");
														setModelOpen(false);
													}}
												>
													Default
													<Check
														className={cn(
															"ml-auto",
															model === "" ? "opacity-100" : "opacity-0",
														)}
													/>
												</CommandItem>
												{modelsData?.models.map((m) => (
													<CommandItem
														key={m.id}
														value={m.id}
														keywords={[m.name]}
														onSelect={(value) => {
															setModel(value === model ? "" : value);
															setModelOpen(false);
														}}
													>
														<div className="flex flex-col">
															<span>{m.name}</span>
															<span className="text-xs text-muted-foreground">
																{m.id}
																{" — "}$
																{(Number(m.promptPricing) * 1_000_000).toFixed(
																	2,
																)}{" "}
																/ $
																{(
																	Number(m.completionPricing) * 1_000_000
																).toFixed(2)}{" "}
																per M tokens
															</span>
														</div>
														<Check
															className={cn(
																"ml-auto shrink-0",
																model === m.id ? "opacity-100" : "opacity-0",
															)}
														/>
													</CommandItem>
												))}
											</CommandGroup>
										</CommandList>
									</Command>
								</PopoverContent>
							</Popover>
							<p className="text-xs text-muted-foreground">
								Leave as "Default" to use the server's default model
							</p>
						</div>

						<div className="space-y-2">
							<Label>Tools</Label>
							<div className="space-y-3">
								<div className="flex items-center space-x-2">
									<Checkbox
										id="tool-fetch"
										checked={enabledTools.includes("fetch_url")}
										onCheckedChange={() => toggleTool("fetch_url")}
									/>
									<label htmlFor="tool-fetch" className="text-sm leading-none">
										URL Fetch
										<span className="ml-2 text-xs text-muted-foreground">
											— Fetch web pages and API responses
										</span>
									</label>
								</div>
								<div className="flex items-center space-x-2">
									<Checkbox
										id="tool-bash"
										checked={enabledTools.includes("bash")}
										onCheckedChange={() => toggleTool("bash")}
									/>
									<label htmlFor="tool-bash" className="text-sm leading-none">
										Bash
										<span className="ml-2 text-xs text-muted-foreground">
											— Run shell commands, Python, JavaScript
										</span>
									</label>
								</div>
								<div className="flex items-center space-x-2">
									<Checkbox
										id="tool-schedule"
										checked={enabledTools.includes("schedule_agent_run")}
										onCheckedChange={() => toggleTool("schedule_agent_run")}
									/>
									<label
										htmlFor="tool-schedule"
										className="text-sm leading-none"
									>
										Schedule Agent Run
										<span className="ml-2 text-xs text-muted-foreground">
											— Schedule future or recurring agent runs
										</span>
									</label>
								</div>
								<div className="flex items-center space-x-2">
									<Checkbox
										id="tool-call"
										checked={enabledTools.includes("call_agent")}
										onCheckedChange={() => toggleTool("call_agent")}
									/>
									<label htmlFor="tool-call" className="text-sm leading-none">
										Call Agent
										<span className="ml-2 text-xs text-muted-foreground">
											— Invoke other agents as subagents
										</span>
									</label>
								</div>
							</div>
						</div>

						{channelsData?.channels && channelsData.channels.length > 0 && (
							<div className="space-y-2">
								<Label>Notification Channels</Label>
								<div className="space-y-3">
									{channelsData.channels.map((channel) => (
										<div
											key={channel.id}
											className="flex items-center space-x-2"
										>
											<Checkbox
												id={`channel-${channel.id}`}
												checked={enabledNotificationChannels.includes(
													channel.id,
												)}
												onCheckedChange={() =>
													toggleNotificationChannel(channel.id)
												}
											/>
											<label
												htmlFor={`channel-${channel.id}`}
												className="text-sm leading-none"
											>
												{channel.name}
												{channel.description && (
													<span className="ml-2 text-xs text-muted-foreground">
														— {channel.description}
													</span>
												)}
											</label>
										</div>
									))}
								</div>
							</div>
						)}

						<Button type="submit" disabled={updateMutation.isPending}>
							{updateMutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</form>
				</CardContent>
			</Card>
		</PageContent>
	);
}
