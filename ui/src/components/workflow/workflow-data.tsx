import React from "react";
import { WorkflowTask } from "./gantt-chart";

export type WorkflowStatus = "ongoing" | "completed" | "pending";

// Mock data for demonstration purposes
export const mockWorkflows = [
  {
    id: "workflow-1",
    name: "Order Processing",
    description: "End-to-end order processing workflow",
    status: "ongoing" as WorkflowStatus,
    tasks: [
      {
        id: "1",
        name: "Receive Order",
        resource: "Order System",
        startDate: new Date(2025, 4, 31, 10, 0, 0), // May 31, 2025, 10:00:00
        endDate: new Date(2025, 4, 31, 10, 0, 30), // May 31, 2025, 10:00:30 (30 seconds)
        percentComplete: 100,
      },
      {
        id: "2",
        name: "Validate Payment",
        resource: "Payment Service",
        startDate: new Date(2025, 4, 31, 10, 0, 30), // May 31, 2025, 10:00:30
        endDate: new Date(2025, 4, 31, 10, 1, 45), // May 31, 2025, 10:01:45 (75 seconds)
        percentComplete: 100,
        dependencies: ["1"],
      },
      {
        id: "3",
        name: "Check Inventory",
        resource: "Inventory System",
        startDate: new Date(2025, 4, 31, 10, 1, 45), // May 31, 2025, 10:01:45
        endDate: new Date(2025, 4, 31, 10, 3, 15), // May 31, 2025, 10:03:15 (90 seconds)
        percentComplete: 80,
        dependencies: ["2"],
      },
      {
        id: "4",
        name: "Prepare Shipment",
        resource: "Warehouse",
        startDate: new Date(2025, 4, 31, 10, 3, 15), // May 31, 2025, 10:03:15
        endDate: new Date(2025, 4, 31, 10, 5, 45), // May 31, 2025, 10:05:45 (150 seconds)
        percentComplete: 30,
        dependencies: ["3"],
      },
      {
        id: "5",
        name: "Deliver Order",
        resource: "Logistics",
        startDate: new Date(2025, 4, 31, 10, 5, 45), // May 31, 2025, 10:05:45
        endDate: new Date(2025, 4, 31, 10, 10, 45), // May 31, 2025, 10:10:45 (300 seconds / 5 minutes)
        percentComplete: 0,
        dependencies: ["4"],
      },
    ],
  },
  {
    id: "workflow-2",
    name: "Customer Onboarding",
    description: "New customer onboarding process",
    status: "ongoing" as WorkflowStatus,
    tasks: [
      {
        id: "1",
        name: "Account Creation",
        resource: "Registration Service",
        startDate: new Date(2025, 4, 31, 11, 0, 0), // May 31, 2025, 11:00:00
        endDate: new Date(2025, 4, 31, 11, 1, 30), // May 31, 2025, 11:01:30 (90 seconds)
        percentComplete: 100,
      },
      {
        id: "2",
        name: "Identity Verification",
        resource: "KYC Service",
        startDate: new Date(2025, 4, 31, 11, 1, 30), // May 31, 2025, 11:01:30
        endDate: new Date(2025, 4, 31, 11, 5, 0), // May 31, 2025, 11:05:00 (210 seconds / 3.5 minutes)
        percentComplete: 70,
        dependencies: ["1"],
      },
      {
        id: "3",
        name: "Email Verification",
        resource: "Notification Service",
        startDate: new Date(2025, 4, 31, 11, 1, 30), // May 31, 2025, 11:01:30
        endDate: new Date(2025, 4, 31, 11, 3, 30), // May 31, 2025, 11:03:30 (120 seconds / 2 minutes)
        percentComplete: 100,
        dependencies: ["1"],
      },
      {
        id: "4",
        name: "Training Session",
        resource: "Training Team",
        startDate: new Date(2025, 4, 31, 11, 5, 0), // May 31, 2025, 11:05:00
        endDate: new Date(2025, 4, 31, 11, 15, 0), // May 31, 2025, 11:15:00 (600 seconds / 10 minutes)
        percentComplete: 0,
        dependencies: ["2", "3"],
      },
    ],
  },
  {
    id: "workflow-3",
    name: "CI/CD Pipeline",
    description: "Continuous integration and deployment workflow",
    status: "completed" as WorkflowStatus,
    tasks: [
      {
        id: "1",
        name: "Code Commit",
        resource: "Developer",
        startDate: new Date(2025, 4, 31, 14, 0, 0), // May 31, 2025, 14:00:00
        endDate: new Date(2025, 4, 31, 14, 0, 45), // May 31, 2025, 14:00:45 (45 seconds)
        percentComplete: 100,
      },
      {
        id: "2",
        name: "Unit Tests",
        resource: "Test Runner",
        startDate: new Date(2025, 4, 31, 14, 0, 45), // May 31, 2025, 14:00:45
        endDate: new Date(2025, 4, 31, 14, 3, 45), // May 31, 2025, 14:03:45 (180 seconds / 3 minutes)
        percentComplete: 100,
        dependencies: ["1"],
      },
      {
        id: "3",
        name: "Integration Tests",
        resource: "Test Runner",
        startDate: new Date(2025, 4, 31, 14, 3, 45), // May 31, 2025, 14:03:45
        endDate: new Date(2025, 4, 31, 14, 8, 45), // May 31, 2025, 14:08:45 (300 seconds / 5 minutes)
        percentComplete: 75,
        dependencies: ["2"],
      },
      {
        id: "4",
        name: "Build Artifacts",
        resource: "Build Server",
        startDate: new Date(2025, 4, 31, 14, 8, 45), // May 31, 2025, 14:08:45
        endDate: new Date(2025, 4, 31, 14, 12, 45), // May 31, 2025, 14:12:45 (240 seconds / 4 minutes)
        percentComplete: 0,
        dependencies: ["3"],
      },
      {
        id: "5",
        name: "Deployment",
        resource: "Deploy Service",
        startDate: new Date(2025, 4, 31, 14, 12, 45), // May 31, 2025, 14:12:45
        endDate: new Date(2025, 4, 31, 14, 18, 45), // May 31, 2025, 14:18:45 (360 seconds / 6 minutes)
        percentComplete: 0,
        dependencies: ["4"],
      },
    ],
  },
];

export interface WorkflowData {
  id: string;
  name: string;
  description: string;
  tasks: WorkflowTask[];
  status: WorkflowStatus;
}

export const getWorkflowById = (id: string): WorkflowData | undefined => {
  return mockWorkflows.find((workflow) => workflow.id === id);
};

export const getAllWorkflows = (): WorkflowData[] => {
  return mockWorkflows;
};
