import { useQuery } from "@connectrpc/connect-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { HardDrive, Plus } from "lucide-react";
import { EmptyState } from "@/components/empty-state";
import { PageContent } from "@/components/page-content";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { listFilesystemRoots } from "@/lib/rpc/fsroot/fsroot-FilesystemRootService_connectquery";

export const Route = createFileRoute("/roots/")({
	component: RootsIndex,
});

function RootsIndex() {
	const { data, isLoading, error } = useQuery(listFilesystemRoots, {});

	if (error) {
		return (
			<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-destructive">
				Error: {error.message}
			</div>
		);
	}

	const roots = data?.roots ?? [];

	return (
		<PageContent className="space-y-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-bold tracking-tight">
						Filesystem Roots
					</h1>
					<p className="text-muted-foreground">
						Configure directories agents can access
					</p>
				</div>
				<Button asChild>
					<Link to="/roots/new">
						<Plus className="h-4 w-4" />
						New Root
					</Link>
				</Button>
			</div>

			{isLoading ? (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{[...Array(3)].map((_, i) => (
						// biome-ignore lint/suspicious/noArrayIndexKey: Static skeleton placeholders never reorder
						<Card key={`skeleton-${i}`}>
							<CardHeader>
								<Skeleton className="h-5 w-32" />
								<Skeleton className="h-4 w-48" />
							</CardHeader>
						</Card>
					))}
				</div>
			) : roots.length === 0 ? (
				<EmptyState
					icon={<HardDrive />}
					title="No filesystem roots"
					description="Add a root to let agents read and edit files"
					action={
						<Button asChild>
							<Link to="/roots/new">
								<Plus className="h-4 w-4" />
								Add Root
							</Link>
						</Button>
					}
				/>
			) : (
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{roots.map((root) => (
						<Card
							key={root.id}
							className="group transition-colors hover:border-foreground/20"
						>
							<CardHeader>
								<div className="space-y-1">
									<CardTitle className="text-base">
										<Link
											to="/roots/$rootId"
											params={{ rootId: root.id }}
											className="hover:underline"
										>
											{root.name}
										</Link>
									</CardTitle>
									<CardDescription className="font-mono text-xs">
										{root.path}
									</CardDescription>
								</div>
							</CardHeader>
						</Card>
					))}
				</div>
			)}
		</PageContent>
	);
}
