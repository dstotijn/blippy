import { useMutation, useQuery } from "@connectrpc/connect-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Trash2 } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
	deleteFilesystemRoot,
	getFilesystemRoot,
	updateFilesystemRoot,
} from "@/lib/rpc/fsroot/fsroot-FilesystemRootService_connectquery";

export const Route = createFileRoute("/roots/$rootId")({
	component: RootDetail,
});

function RootDetail() {
	const { rootId } = Route.useParams();
	const navigate = useNavigate();
	const { data: root, isLoading } = useQuery(getFilesystemRoot, {
		id: rootId,
	});
	const updateMutation = useMutation(updateFilesystemRoot);
	const deleteMutation = useMutation(deleteFilesystemRoot);

	const [name, setName] = useState("");
	const [path, setPath] = useState("");
	const [description, setDescription] = useState("");

	useEffect(() => {
		if (root) {
			setName(root.name);
			setPath(root.path);
			setDescription(root.description);
		}
	}, [root]);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		try {
			await updateMutation.mutateAsync({ id: rootId, name, path, description });
			toast.success("Root updated");
		} catch {
			toast.error("Failed to update root");
		}
	};

	const handleDelete = async () => {
		if (!confirm("Are you sure you want to delete this root?")) return;
		try {
			await deleteMutation.mutateAsync({ id: rootId });
			toast.success("Root deleted");
			navigate({ to: "/roots" });
		} catch {
			toast.error("Failed to delete root");
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

	if (!root) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Root not found
			</div>
		);
	}

	return (
		<PageContent className="mx-auto max-w-2xl space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">{root.name}</h1>
					<p className="text-muted-foreground">Edit root settings</p>
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
					<CardTitle>Root Settings</CardTitle>
					<CardDescription>Update the root configuration</CardDescription>
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
							<Label htmlFor="path">Path</Label>
							<Input
								id="path"
								value={path}
								onChange={(e) => setPath(e.target.value)}
								className="font-mono"
								required
							/>
						</div>

						<div className="space-y-2">
							<Label htmlFor="description">Description</Label>
							<Textarea
								id="description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Describe what this directory contains"
								rows={2}
							/>
							<p className="text-xs text-muted-foreground">
								Helps agents understand when and how to use this root
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
