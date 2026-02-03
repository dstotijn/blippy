import { useQuery } from "@connectrpc/connect-query";
import { Link, useRouterState } from "@tanstack/react-router";
import { AppSidebar } from "@/components/app-sidebar";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Separator } from "@/components/ui/separator";
import {
	SidebarInset,
	SidebarProvider,
	SidebarTrigger,
} from "@/components/ui/sidebar";
import { getAgent } from "@/lib/rpc/agent/agent-AgentService_connectquery";

interface LayoutProps {
	children: React.ReactNode;
}

function useBreadcrumbs() {
	const routerState = useRouterState();
	const pathname = routerState.location.pathname;
	const segments = pathname.split("/").filter(Boolean);

	// Extract agent ID if on an agent route
	const agentId =
		segments[0] === "agents" && segments[1] && segments[1] !== "new"
			? segments[1]
			: undefined;

	const { data: agent } = useQuery(
		getAgent,
		{ id: agentId ?? "" },
		{ enabled: !!agentId },
	);

	const breadcrumbs: { label: string; href?: string }[] = [];

	if (segments.length === 0) {
		breadcrumbs.push({ label: "Agents" });
	} else if (segments[0] === "agents") {
		breadcrumbs.push({ label: "Agents", href: "/" });
		if (segments[1] === "new") {
			breadcrumbs.push({ label: "New Agent" });
		} else if (segments[1]) {
			const agentName = agent?.name ?? "Agent";
			breadcrumbs.push({ label: agentName, href: `/agents/${segments[1]}` });
			if (segments[2] === "settings") {
				breadcrumbs.push({ label: "Settings" });
			} else if (segments[2]) {
				breadcrumbs.push({ label: "Chat" });
			}
		}
	} else if (segments[0] === "triggers") {
		breadcrumbs.push({ label: "Triggers", href: "/triggers" });
		if (segments[1] === "new") {
			breadcrumbs.push({ label: "New Trigger" });
		} else if (segments[1]) {
			breadcrumbs.push({ label: "Trigger" });
		}
	} else if (segments[0] === "notifications") {
		breadcrumbs.push({ label: "Notifications", href: "/notifications" });
		if (segments[1] === "new") {
			breadcrumbs.push({ label: "New Channel" });
		} else if (segments[1]) {
			breadcrumbs.push({ label: "Channel" });
		}
	}

	return breadcrumbs;
}

export function Layout({ children }: LayoutProps) {
	const breadcrumbs = useBreadcrumbs();

	return (
		<SidebarProvider>
			<AppSidebar />
			<SidebarInset>
				<header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
					<SidebarTrigger className="-ml-1" />
					<Separator orientation="vertical" className="mr-2 h-4" />
					<Breadcrumb>
						<BreadcrumbList>
							{breadcrumbs.map((crumb, index) => (
								<BreadcrumbItem key={crumb.label}>
									{index > 0 && <BreadcrumbSeparator />}
									{crumb.href ? (
										<BreadcrumbLink asChild>
											<Link to={crumb.href}>{crumb.label}</Link>
										</BreadcrumbLink>
									) : (
										<BreadcrumbPage>{crumb.label}</BreadcrumbPage>
									)}
								</BreadcrumbItem>
							))}
						</BreadcrumbList>
					</Breadcrumb>
				</header>
				<main className="flex-1 overflow-auto p-4 md:p-6">{children}</main>
			</SidebarInset>
		</SidebarProvider>
	);
}
