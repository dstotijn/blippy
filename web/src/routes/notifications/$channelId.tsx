import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
	deleteNotificationChannel,
	getNotificationChannel,
	updateNotificationChannel,
} from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";

export const Route = createFileRoute("/notifications/$channelId")({
	component: NotificationChannelDetail,
});

function NotificationChannelDetail() {
	const { channelId } = Route.useParams();
	const navigate = useNavigate();
	const { data: channel, isLoading } = useQuery(getNotificationChannel, {
		id: channelId,
	});
	const updateMutation = useMutation(updateNotificationChannel);
	const deleteMutation = useMutation(deleteNotificationChannel);

	const [name, setName] = useState("");
	const [type, setType] = useState("http_request");
	const [url, setUrl] = useState("");
	const [method, setMethod] = useState("POST");
	const [headers, setHeaders] = useState("");
	const [description, setDescription] = useState("");
	const [jsonSchema, setJsonSchema] = useState("");

	const parsedConfig = useMemo(() => {
		if (!channel?.config) return {};
		try {
			return JSON.parse(channel.config);
		} catch {
			return {};
		}
	}, [channel?.config]);

	useEffect(() => {
		if (channel) {
			setName(channel.name);
			setType(channel.type);
			setUrl(parsedConfig.url ?? "");
			setMethod(parsedConfig.method ?? "POST");
			// Convert headers object back to "Key: Value" format
			const hdrs = parsedConfig.headers ?? {};
			setHeaders(
				Object.entries(hdrs)
					.map(([k, v]) => `${k}: ${v}`)
					.join("\n"),
			);
			setDescription(channel.description ?? "");
			setJsonSchema(channel.jsonSchema ?? "");
		}
	}, [channel, parsedConfig]);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		// Parse headers from "Key: Value" format
		const headerObj: Record<string, string> = {};
		if (headers.trim()) {
			for (const line of headers.split("\n")) {
				const idx = line.indexOf(":");
				if (idx > 0) {
					const key = line.slice(0, idx).trim();
					const value = line.slice(idx + 1).trim();
					if (key) headerObj[key] = value;
				}
			}
		}

		const config = JSON.stringify({ url, method, headers: headerObj });
		try {
			await updateMutation.mutateAsync({
				id: channelId,
				name,
				type,
				config,
				description,
				jsonSchema,
			});
			toast.success("Channel updated");
		} catch {
			toast.error("Failed to update channel");
		}
	};

	const handleDelete = async () => {
		if (!confirm("Are you sure you want to delete this channel?")) return;
		try {
			await deleteMutation.mutateAsync({ id: channelId });
			toast.success("Channel deleted");
			navigate({ to: "/notifications" });
		} catch {
			toast.error("Failed to delete channel");
		}
	};

	if (isLoading) {
		return (
			<PageContent className="mx-auto max-w-2xl space-y-6">
				<Skeleton className="h-8 w-48" />
				<Card>
					<CardHeader>
						<Skeleton className="h-6 w-32" />
					</CardHeader>
					<CardContent className="space-y-4">
						<Skeleton className="h-10 w-full" />
						<Skeleton className="h-10 w-full" />
					</CardContent>
				</Card>
			</PageContent>
		);
	}

	if (!channel) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Channel not found
			</div>
		);
	}

	return (
		<PageContent className="mx-auto max-w-2xl space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">{channel.name}</h1>
					<p className="text-muted-foreground">Edit channel settings</p>
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
					<CardTitle>Channel Settings</CardTitle>
					<CardDescription>Update the channel configuration</CardDescription>
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
							<Label htmlFor="type">Type</Label>
							<Select value={type} onValueChange={setType}>
								<SelectTrigger id="type">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="http_request">HTTP Request</SelectItem>
								</SelectContent>
							</Select>
						</div>

						{type === "http_request" && (
							<>
								<div className="space-y-2">
									<Label htmlFor="url">URL</Label>
									<Input
										id="url"
										type="url"
										value={url}
										onChange={(e) => setUrl(e.target.value)}
										required
									/>
								</div>

								<div className="space-y-2">
									<Label htmlFor="method">Method</Label>
									<Select value={method} onValueChange={setMethod}>
										<SelectTrigger id="method">
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											<SelectItem value="POST">POST</SelectItem>
											<SelectItem value="PUT">PUT</SelectItem>
											<SelectItem value="PATCH">PATCH</SelectItem>
										</SelectContent>
									</Select>
								</div>

								<div className="space-y-2">
									<Label htmlFor="headers">Headers</Label>
									<Textarea
										id="headers"
										value={headers}
										onChange={(e) => setHeaders(e.target.value)}
										placeholder={"Authorization: Bearer token"}
										rows={3}
									/>
									<p className="text-xs text-muted-foreground">
										One header per line in "Key: Value" format
									</p>
								</div>
							</>
						)}

						<div className="space-y-2">
							<Label htmlFor="description">Description</Label>
							<Textarea
								id="description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Describe when agents should use this channel (e.g., 'Send critical production alerts here')"
								rows={2}
							/>
							<p className="text-xs text-muted-foreground">
								Helps agents understand when to use this channel
							</p>
						</div>

						<div className="space-y-2">
							<Label htmlFor="jsonSchema">JSON Schema</Label>
							<Textarea
								id="jsonSchema"
								value={jsonSchema}
								onChange={(e) => setJsonSchema(e.target.value)}
								placeholder={`{
  "type": "object",
  "properties": {
    "text": { "type": "string" }
  },
  "required": ["text"]
}`}
								rows={6}
								className="font-mono text-sm"
							/>
							<p className="text-xs text-muted-foreground">
								JSON Schema for the notification payload (optional)
							</p>
						</div>

						<Button type="submit" disabled={updateMutation.isPending}>
							{updateMutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</form>
				</CardContent>
			</Card>
		</PageContent>
	);
}
