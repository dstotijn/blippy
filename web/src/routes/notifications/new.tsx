import { useMutation } from "@connectrpc/connect-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
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
import { Textarea } from "@/components/ui/textarea";
import { createNotificationChannel } from "@/lib/rpc/notification/notification-NotificationChannelService_connectquery";

export const Route = createFileRoute("/notifications/new")({
	component: NewNotificationChannel,
});

function NewNotificationChannel() {
	const navigate = useNavigate();
	const mutation = useMutation(createNotificationChannel);

	const [name, setName] = useState("");
	const [type, setType] = useState("http_request");
	const [url, setUrl] = useState("");
	const [method, setMethod] = useState("POST");
	const [headers, setHeaders] = useState("");
	const [description, setDescription] = useState("");
	const [jsonSchema, setJsonSchema] = useState("");

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		// Parse headers from "Key: Value" format (one per line)
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
			const channel = await mutation.mutateAsync({
				name,
				type,
				config,
				description,
				jsonSchema,
			});
			toast.success("Channel created");
			navigate({
				to: "/notifications/$channelId",
				params: { channelId: channel.id },
			});
		} catch {
			toast.error("Failed to create channel");
		}
	};

	return (
		<PageContent className="mx-auto max-w-2xl space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">New Channel</h1>
				<p className="text-muted-foreground">
					Add a notification channel for agents
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Channel Details</CardTitle>
					<CardDescription>
						Configure the notification destination
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
								placeholder="e.g., alerts, team-updates"
								required
							/>
							<p className="text-xs text-muted-foreground">
								Agents use this name to send notifications
							</p>
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
										placeholder="https://hooks.slack.com/..."
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
										placeholder={
											"Authorization: Bearer token\nContent-Type: application/json"
										}
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

						<div className="flex gap-3">
							<Button type="submit" disabled={mutation.isPending}>
								{mutation.isPending ? "Creating..." : "Create Channel"}
							</Button>
							<Button variant="outline" asChild>
								<Link to="/notifications">Cancel</Link>
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</PageContent>
	);
}
