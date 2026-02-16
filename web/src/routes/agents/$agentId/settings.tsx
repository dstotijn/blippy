import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Check, ChevronsUpDown, Trash2, X } from "lucide-react";
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
	deleteAgent,
	getAgent,
	listModels,
	updateAgent,
} from "@/lib/rpc/agent/agent-AgentService_connectquery";
import { listFilesystemRoots } from "@/lib/rpc/fsroot/fsroot-FilesystemRootService_connectquery";
import { listNotificationChannels } from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/agents/$agentId/settings")({
	component: AgentPage,
});

const fsTools = [
	{ name: "fs_view", label: "View" },
	{ name: "fs_create", label: "Create" },
	{ name: "fs_str_replace", label: "Replace" },
	{ name: "fs_insert", label: "Insert" },
] as const;

function AgentPage() {
	const { agentId } = Route.useParams();
	const navigate = useNavigate();
	const { data: agent, isLoading, error } = useQuery(getAgent, { id: agentId });
	const { data: channelsData } = useQuery(listNotificationChannels, {});
	const { data: rootsData } = useQuery(listFilesystemRoots, {});
	const { data: modelsData } = useQuery(listModels, {});
	const updateMutation = useMutation(updateAgent);
	const deleteMutation = useMutation(deleteAgent);

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [systemPrompt, setSystemPrompt] = useState("");
	const [enabledTools, setEnabledTools] = useState<string[]>([]);
	const [enabledNotificationChannels, setEnabledNotificationChannels] =
		useState<string[]>([]);
	const [enabledFilesystemRoots, setEnabledFilesystemRoots] = useState<
		{ rootId: string; enabledTools: string[] }[]
	>([]);
	const [model, setModel] = useState("");
	const [modelOpen, setModelOpen] = useState(false);
	const [forwardedHostEnvVars, setForwardedHostEnvVars] = useState<string[]>(
		[],
	);
	const [newEnvVar, setNewEnvVar] = useState("");

	useEffect(() => {
		if (agent) {
			setName(agent.name);
			setDescription(agent.description);
			setSystemPrompt(agent.systemPrompt);
			setEnabledTools(agent.enabledTools || []);
			setEnabledNotificationChannels(agent.enabledNotificationChannels || []);
			setEnabledFilesystemRoots(
				agent.enabledFilesystemRoots?.map((r) => ({
					rootId: r.rootId,
					enabledTools: [...r.enabledTools],
				})) || [],
			);
			setModel(agent.model);
			setForwardedHostEnvVars(agent.forwardedHostEnvVars || []);
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
				enabledFilesystemRoots,
				model,
				forwardedHostEnvVars,
			});
			toast.success("Agent updated");
		} catch {
			toast.error("Failed to update agent");
		}
	};

	const handleDelete = async () => {
		if (
			!confirm(
				"Are you sure you want to delete this agent? All conversations, messages, and triggers will be permanently deleted.",
			)
		)
			return;
		try {
			await deleteMutation.mutateAsync({ id: agentId });
			toast.success("Agent deleted");
			navigate({ to: "/" });
		} catch {
			toast.error("Failed to delete agent");
		}
	};

	const memoryTools = [
		"memory_view",
		"memory_create",
		"memory_edit",
		"memory_delete",
	];
	const memoryEnabled = memoryTools.every((t) => enabledTools.includes(t));

	const toggleTool = (toolName: string) => {
		setEnabledTools((prev) =>
			prev.includes(toolName)
				? prev.filter((t) => t !== toolName)
				: [...prev, toolName],
		);
	};

	const toggleMemory = () => {
		setEnabledTools((prev) =>
			memoryEnabled
				? prev.filter((t) => !memoryTools.includes(t))
				: [...prev.filter((t) => !memoryTools.includes(t)), ...memoryTools],
		);
	};

	const toggleNotificationChannel = (channelId: string) => {
		setEnabledNotificationChannels((prev) =>
			prev.includes(channelId)
				? prev.filter((id) => id !== channelId)
				: [...prev, channelId],
		);
	};

	const toggleRootTool = (rootId: string, toolName: string) => {
		setEnabledFilesystemRoots((prev) => {
			const existing = prev.find((r) => r.rootId === rootId);
			if (!existing) {
				return [...prev, { rootId, enabledTools: [toolName] }];
			}
			const hasTool = existing.enabledTools.includes(toolName);
			const newTools = hasTool
				? existing.enabledTools.filter((t) => t !== toolName)
				: [...existing.enabledTools, toolName];
			if (newTools.length === 0) {
				return prev.filter((r) => r.rootId !== rootId);
			}
			return prev.map((r) =>
				r.rootId === rootId ? { ...r, enabledTools: newTools } : r,
			);
		});
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
								<div className="flex items-center space-x-2">
									<Checkbox
										id="tool-memory"
										checked={memoryEnabled}
										onCheckedChange={toggleMemory}
									/>
									<label htmlFor="tool-memory" className="text-sm leading-none">
										Memory
										<span className="ml-2 text-xs text-muted-foreground">
											— Remember information across conversations
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

						{rootsData?.roots && rootsData.roots.length > 0 && (
							<div className="space-y-2">
								<Label>Filesystem Roots</Label>
								<div className="space-y-3">
									{rootsData.roots.map((root) => {
										const config = enabledFilesystemRoots.find(
											(r) => r.rootId === root.id,
										);
										return (
											<div
												key={root.id}
												className="space-y-2 rounded-lg border p-3"
											>
												<div className="text-sm font-medium">
													{root.name}
													<span className="ml-2 font-normal text-xs text-muted-foreground">
														— {root.path}
													</span>
												</div>
												<div className="grid grid-cols-2 gap-2">
													{fsTools.map((tool) => (
														<div
															key={tool.name}
															className="flex items-center space-x-2"
														>
															<Checkbox
																id={`root-${root.id}-${tool.name}`}
																checked={
																	config?.enabledTools.includes(tool.name) ??
																	false
																}
																onCheckedChange={() =>
																	toggleRootTool(root.id, tool.name)
																}
															/>
															<label
																htmlFor={`root-${root.id}-${tool.name}`}
																className="text-sm leading-none"
															>
																{tool.label}
															</label>
														</div>
													))}
												</div>
											</div>
										);
									})}
								</div>
							</div>
						)}

						<div className="space-y-2">
							<Label>Environment Variables</Label>
							<p className="text-xs text-muted-foreground">
								Host environment variable names to forward into bash execution
							</p>
							<div className="flex gap-2">
								<Input
									type="text"
									placeholder="ENV_VAR_NAME"
									value={newEnvVar}
									onChange={(e) =>
										setNewEnvVar(
											e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ""),
										)
									}
									onKeyDown={(e) => {
										if (e.key === "Enter") {
											e.preventDefault();
											const trimmed = newEnvVar.trim();
											if (trimmed && !forwardedHostEnvVars.includes(trimmed)) {
												setForwardedHostEnvVars((prev) => [...prev, trimmed]);
											}
											setNewEnvVar("");
										}
									}}
									className="font-mono"
								/>
								<Button
									type="button"
									variant="secondary"
									onClick={() => {
										const trimmed = newEnvVar.trim();
										if (trimmed && !forwardedHostEnvVars.includes(trimmed)) {
											setForwardedHostEnvVars((prev) => [...prev, trimmed]);
										}
										setNewEnvVar("");
									}}
								>
									Add
								</Button>
							</div>
							{forwardedHostEnvVars.length > 0 && (
								<div className="flex flex-wrap gap-2">
									{forwardedHostEnvVars.map((envVar) => (
										<span
											key={envVar}
											className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-1 font-mono text-xs"
										>
											{envVar}
											<button
												type="button"
												onClick={() =>
													setForwardedHostEnvVars((prev) =>
														prev.filter((v) => v !== envVar),
													)
												}
												className="text-muted-foreground hover:text-foreground"
											>
												<X className="h-3 w-3" />
											</button>
										</span>
									))}
								</div>
							)}
						</div>

						<Button type="submit" disabled={updateMutation.isPending}>
							{updateMutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</form>
				</CardContent>
			</Card>

			<Card className="border-destructive/50">
				<CardHeader>
					<CardTitle>Danger Zone</CardTitle>
					<CardDescription>
						Permanently delete this agent and all its data
					</CardDescription>
				</CardHeader>
				<CardContent>
					<p className="mb-4 text-sm text-muted-foreground">
						This will permanently delete the agent, including all conversations,
						messages, and triggers. This action cannot be undone.
					</p>
					<Button
						variant="destructive"
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						<Trash2 className="mr-2 h-4 w-4" />
						{deleteMutation.isPending ? "Deleting..." : "Delete Agent"}
					</Button>
				</CardContent>
			</Card>
		</PageContent>
	);
}
