import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Check, ChevronsUpDown } from "lucide-react";
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
import { Textarea } from "@/components/ui/textarea";
import {
	createAgent,
	listModels,
} from "@/lib/rpc/agent/agent-AgentService_connectquery";
import { listNotificationChannels } from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/agents/new")({
	component: NewAgent,
});

function NewAgent() {
	const navigate = useNavigate();
	const mutation = useMutation(createAgent);
	const { data: channelsData } = useQuery(listNotificationChannels, {});
	const { data: modelsData } = useQuery(listModels, {});

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [systemPrompt, setSystemPrompt] = useState("");
	const [enabledTools, setEnabledTools] = useState<string[]>([]);
	const [enabledNotificationChannels, setEnabledNotificationChannels] =
		useState<string[]>([]);
	const [model, setModel] = useState("");
	const [modelOpen, setModelOpen] = useState(false);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			const agent = await mutation.mutateAsync({
				name,
				description,
				systemPrompt,
				enabledTools,
				enabledNotificationChannels,
				model,
			});
			toast.success("Agent created");
			navigate({ to: "/agents/$agentId", params: { agentId: agent.id } });
		} catch {
			toast.error("Failed to create agent");
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

	return (
		<div className="mx-auto max-w-2xl space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">New Agent</h1>
				<p className="text-muted-foreground">
					Create a new AI agent with custom instructions
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Agent Details</CardTitle>
					<CardDescription>
						Configure your agent's name, description, and behavior
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
								placeholder="e.g., Code Assistant"
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
								placeholder="Brief description of what this agent does"
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="systemPrompt">System Prompt</Label>
							<Textarea
								id="systemPrompt"
								value={systemPrompt}
								onChange={(e) => setSystemPrompt(e.target.value)}
								placeholder="Instructions for how the agent should behave..."
								rows={6}
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

						<div className="flex gap-3">
							<Button type="submit" disabled={mutation.isPending}>
								{mutation.isPending ? "Creating..." : "Create Agent"}
							</Button>
							<Button variant="outline" asChild>
								<Link to="/">Cancel</Link>
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</div>
	);
}
