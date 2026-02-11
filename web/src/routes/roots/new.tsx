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
import { Textarea } from "@/components/ui/textarea";
import { createFilesystemRoot } from "@/lib/rpc/fsroot/fsroot-FilesystemRootService_connectquery";

export const Route = createFileRoute("/roots/new")({
	component: NewRoot,
});

function NewRoot() {
	const navigate = useNavigate();
	const mutation = useMutation(createFilesystemRoot);

	const [name, setName] = useState("");
	const [path, setPath] = useState("");
	const [description, setDescription] = useState("");

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			const root = await mutation.mutateAsync({ name, path, description });
			toast.success("Root created");
			navigate({ to: "/roots/$rootId", params: { rootId: root.id } });
		} catch {
			toast.error("Failed to create root");
		}
	};

	return (
		<PageContent className="mx-auto max-w-2xl space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">New Root</h1>
				<p className="text-muted-foreground">
					Add a filesystem root for agents to access
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Root Details</CardTitle>
					<CardDescription>Configure the filesystem directory</CardDescription>
				</CardHeader>
				<CardContent>
					<form onSubmit={handleSubmit} className="space-y-6">
						<div className="space-y-2">
							<Label htmlFor="name">Name</Label>
							<Input
								id="name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="e.g., obsidian, notes, config"
								required
							/>
							<p className="text-xs text-muted-foreground">
								Identifier agents use to reference this root
							</p>
						</div>

						<div className="space-y-2">
							<Label htmlFor="path">Path</Label>
							<Input
								id="path"
								value={path}
								onChange={(e) => setPath(e.target.value)}
								placeholder="/home/user/documents"
								className="font-mono"
								required
							/>
							<p className="text-xs text-muted-foreground">
								Absolute path on the host machine
							</p>
						</div>

						<div className="space-y-2">
							<Label htmlFor="description">Description</Label>
							<Textarea
								id="description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Describe what this directory contains (helps the agent understand context)"
								rows={2}
							/>
							<p className="text-xs text-muted-foreground">
								Helps agents understand when and how to use this root
							</p>
						</div>

						<div className="flex gap-3">
							<Button type="submit" disabled={mutation.isPending}>
								{mutation.isPending ? "Creating..." : "Create Root"}
							</Button>
							<Button variant="outline" asChild>
								<Link to="/roots">Cancel</Link>
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</PageContent>
	);
}
