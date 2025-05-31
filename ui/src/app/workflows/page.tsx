"use client";

import React, { useState, useEffect } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import GanttChart from "@/components/workflow/gantt-chart";
import {
  getAllWorkflows,
  getWorkflowById,
} from "@/components/workflow/workflow-data";
import type { WorkflowData } from "@/components/workflow/workflow-data";

export default function WorkflowProgressPage() {
  const [activeTab, setActiveTab] = useState("gantt");
  const [selectedWorkflow, setSelectedWorkflow] = useState<string>("");
  const [workflows, setWorkflows] = useState<WorkflowData[]>([]);
  const [filteredWorkflows, setFilteredWorkflows] = useState<WorkflowData[]>(
    []
  );
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [currentWorkflow, setCurrentWorkflow] = useState<WorkflowData | null>(
    null
  );

  useEffect(() => {
    // Load workflows
    const allWorkflows = getAllWorkflows();
    setWorkflows(allWorkflows);
    setFilteredWorkflows(allWorkflows);

    // Set default selection to the first workflow
    if (allWorkflows.length > 0 && !selectedWorkflow) {
      setSelectedWorkflow(allWorkflows[0].id);
      setCurrentWorkflow(allWorkflows[0]);
    }
  }, []);

  // Filter workflows based on status
  useEffect(() => {
    if (statusFilter === "all") {
      setFilteredWorkflows(workflows);
    } else {
      const filtered = workflows.filter(
        (workflow) => workflow.status === statusFilter
      );
      setFilteredWorkflows(filtered);

      // If current workflow is not in the filtered list, select the first one from filtered list
      if (
        filtered.length > 0 &&
        currentWorkflow &&
        !filtered.some((w) => w.id === currentWorkflow.id)
      ) {
        setSelectedWorkflow(filtered[0].id);
        setCurrentWorkflow(filtered[0]);
      }
    }
  }, [statusFilter, workflows, currentWorkflow]);

  // Handle workflow selection change
  useEffect(() => {
    if (selectedWorkflow) {
      const workflow = getWorkflowById(selectedWorkflow);
      if (workflow) {
        setCurrentWorkflow(workflow);
      }
    }
  }, [selectedWorkflow]);

  // Calculate overall workflow progress
  const calculateProgress = (workflow: WorkflowData | null): number => {
    if (!workflow || workflow.tasks.length === 0) return 0;

    const totalTasks = workflow.tasks.length;
    const completedTasksWeight = workflow.tasks.reduce((sum, task) => {
      return sum + (task.percentComplete || 0) / 100;
    }, 0);

    return Math.round((completedTasksWeight / totalTasks) * 100);
  };

  const overallProgress = calculateProgress(currentWorkflow);

  return (
    <div className="container mx-auto py-8 max-w-7xl">
      <a
        href="https://github.com/ngnhng/diy-temporal"
        target="_blank"
        rel="noopener noreferrer"
        className="fixed top-4 right-4 z-50 p-2 bg-white rounded-full shadow-md hover:shadow-lg transition-shadow duration-300"
        aria-label="GitHub Repository"
        title="View source code on GitHub"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="hover:scale-110 transition-transform duration-200"
        >
          <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
        </svg>
      </a>

      <h1 className="text-3xl font-bold mb-6">
        Workflow Progress Visualization
      </h1>

      <div className="mb-6">
        <div className="flex flex-col sm:flex-row gap-4 sm:items-center mb-4">
          <label className="text-sm font-medium w-40">Status Filter:</label>
          <div className="w-full sm:w-80">
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger>
                <SelectValue placeholder="Filter by status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Statuses</SelectItem>
                <SelectItem value="ongoing">Ongoing</SelectItem>
                <SelectItem value="completed">Completed</SelectItem>
                <SelectItem value="pending">Pending</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-4 sm:items-center mb-4">
          <label className="text-sm font-medium w-40">Select Workflow:</label>
          <div className="w-full sm:w-80">
            <Select
              value={selectedWorkflow}
              onValueChange={setSelectedWorkflow}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a workflow" />
              </SelectTrigger>
              <SelectContent>
                {filteredWorkflows.map((workflow) => (
                  <SelectItem key={workflow.id} value={workflow.id}>
                    {workflow.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      {currentWorkflow && (
        <>
          <Card className="mb-6">
            <CardHeader>
              <div className="flex items-center space-x-2">
                <span
                  className={`inline-block w-3 h-3 rounded-full ${
                    currentWorkflow.status === "completed"
                      ? "bg-green-500"
                      : currentWorkflow.status === "ongoing"
                      ? "bg-blue-500 animate-ping"
                      : "bg-gray-400"
                  }`}
                ></span>
                <CardTitle>{currentWorkflow.name}</CardTitle>
                <span className="text-sm font-medium ml-2 px-2 py-1 rounded-full bg-gray-100 text-gray-800 capitalize">
                  {currentWorkflow.status}
                </span>
              </div>
              <CardDescription>{currentWorkflow.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="mb-4">
                <div className="mb-2 flex justify-between">
                  <span className="font-medium">Overall Progress</span>
                  <span className="font-bold">{overallProgress}%</span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2.5">
                  <div
                    className={`h-2.5 rounded-full ${
                      overallProgress === 100
                        ? "bg-green-600"
                        : overallProgress > 0
                        ? "bg-blue-600 animate-pulse"
                        : "bg-gray-400"
                    }`}
                    style={{ width: `${overallProgress}%` }}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
                {/* Task Summary */}
                <div className="bg-gray-50 p-4 rounded-lg">
                  <h3 className="text-sm font-medium text-gray-500 mb-2">
                    Tasks
                  </h3>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <div className="text-2xl font-bold">
                        {currentWorkflow.tasks.length}
                      </div>
                      <div className="text-xs text-gray-500">Total</div>
                    </div>
                    <div>
                      <div className="text-2xl font-bold">
                        {
                          currentWorkflow.tasks.filter(
                            (t) => (t.percentComplete || 0) === 100
                          ).length
                        }
                      </div>
                      <div className="text-xs text-gray-500">Completed</div>
                    </div>
                  </div>
                </div>

                {/* Duration */}
                <div className="bg-gray-50 p-4 rounded-lg">
                  <h3 className="text-sm font-medium text-gray-500 mb-2">
                    Duration
                  </h3>
                  <div>
                    {(() => {
                      if (currentWorkflow.tasks.length === 0) return "0m 0s";

                      const firstTask = currentWorkflow.tasks[0];
                      const lastTask =
                        currentWorkflow.tasks[currentWorkflow.tasks.length - 1];

                      const totalDurationSec = Math.round(
                        (lastTask.endDate.getTime() -
                          firstTask.startDate.getTime()) /
                          1000
                      );

                      const minutes = Math.floor(totalDurationSec / 60);
                      const seconds = totalDurationSec % 60;

                      return (
                        <div className="text-2xl font-bold">
                          {minutes}m {seconds}s
                        </div>
                      );
                    })()}
                    <div className="text-xs text-gray-500">
                      Total Workflow Time
                    </div>
                  </div>
                </div>

                {/* Status */}
                <div className="bg-gray-50 p-4 rounded-lg">
                  <h3 className="text-sm font-medium text-gray-500 mb-2">
                    Status
                  </h3>
                  <div className="flex items-center">
                    <span
                      className={`inline-block w-3 h-3 rounded-full mr-2 ${
                        currentWorkflow.status === "completed"
                          ? "bg-green-500"
                          : currentWorkflow.status === "ongoing"
                          ? "bg-blue-500 animate-pulse"
                          : "bg-gray-400"
                      }`}
                    ></span>
                    <span className="text-2xl font-bold capitalize">
                      {currentWorkflow.status}
                    </span>
                  </div>
                  <div className="text-xs text-gray-500 mt-1">
                    Current Workflow Status
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Tabs
            defaultValue={activeTab}
            onValueChange={setActiveTab}
            className="w-full"
          >
            <TabsList className="grid w-full sm:w-80 grid-cols-2 mb-6">
              <TabsTrigger value="gantt">Gantt Chart</TabsTrigger>
              <TabsTrigger value="tasks">Task List</TabsTrigger>
            </TabsList>

            <TabsContent value="gantt">
              <GanttChart
                tasks={currentWorkflow.tasks}
                title={`${currentWorkflow.name} - Gantt Chart`}
                description="Timeline view of workflow tasks"
                height={500}
                status={currentWorkflow.status}
              />
            </TabsContent>

            <TabsContent value="tasks">
              <Card>
                <CardHeader>
                  <CardTitle>Tasks for {currentWorkflow.name}</CardTitle>
                  <CardDescription>
                    Detailed list of all tasks in this workflow
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <table className="w-full border-collapse">
                      <thead>
                        <tr className="border-b">
                          <th className="py-2 px-4 text-left">Task Name</th>
                          <th className="py-2 px-4 text-left">Resource</th>
                          <th className="py-2 px-4 text-left">Start Time</th>
                          <th className="py-2 px-4 text-left">End Time</th>
                          <th className="py-2 px-4 text-left">Duration</th>
                          <th className="py-2 px-4 text-left">Status</th>
                          <th className="py-2 px-4 text-left">Progress</th>
                        </tr>
                      </thead>
                      <tbody>
                        {currentWorkflow.tasks.map((task) => {
                          const percentComplete = task.percentComplete || 0;

                          // Calculate the task status based on percentComplete
                          const taskStatus =
                            percentComplete === 100
                              ? "Completed"
                              : percentComplete > 0
                              ? "Ongoing"
                              : "Pending";

                          // Calculate duration in seconds
                          const durationSeconds = Math.round(
                            (task.endDate.getTime() -
                              task.startDate.getTime()) /
                              1000
                          );

                          // Format duration as minutes:seconds
                          const minutes = Math.floor(durationSeconds / 60);
                          const seconds = durationSeconds % 60;
                          const formattedDuration = `${minutes}m ${seconds}s`;

                          // Determine status color
                          const statusColorClass =
                            taskStatus === "Completed"
                              ? "bg-green-100 text-green-800"
                              : taskStatus === "Ongoing"
                              ? "bg-blue-100 text-blue-800"
                              : "bg-gray-100 text-gray-800";

                          return (
                            <tr
                              key={task.id}
                              className="border-b hover:bg-gray-50"
                            >
                              <td className="py-3 px-4 font-medium">
                                {task.name}
                              </td>
                              <td className="py-3 px-4">{task.resource}</td>
                              <td className="py-3 px-4">
                                {task.startDate.toLocaleTimeString([], {
                                  day: "2-digit",
                                  month: "short",
                                  year: "numeric",
                                  hour: "2-digit",
                                  minute: "2-digit",
                                  second: "2-digit",
                                  timeZoneName: "short",
                                })}
                              </td>
                              <td className="py-3 px-4">
                                {task.endDate.toLocaleTimeString([], {
                                  day: "2-digit",
                                  month: "short",
                                  year: "numeric",
                                  hour: "2-digit",
                                  minute: "2-digit",
                                  second: "2-digit",
                                  timeZoneName: "short",
                                })}
                              </td>
                              <td className="py-3 px-4">{formattedDuration}</td>
                              <td className="py-3 px-4">
                                <span
                                  className={`px-2 py-1 rounded-full text-xs font-medium ${statusColorClass}`}
                                >
                                  {taskStatus}
                                </span>
                              </td>
                              <td className="py-3 px-4">
                                <div className="flex items-center gap-2">
                                  <div className="w-full bg-gray-200 rounded-full h-2">
                                    <div
                                      className={`h-2 rounded-full ${
                                        taskStatus === "Completed"
                                          ? "bg-green-600"
                                          : taskStatus === "Ongoing"
                                          ? "bg-blue-600"
                                          : "bg-gray-400"
                                      }`}
                                      style={{
                                        width: `${percentComplete}%`,
                                      }}
                                    />
                                  </div>
                                  <span className="text-xs font-medium">
                                    {percentComplete}%
                                  </span>
                                </div>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  );
}
