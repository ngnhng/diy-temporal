import React from "react";
import { Chart } from "react-google-charts";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { WorkflowStatus } from "./workflow-data";

export interface WorkflowTask {
  id: string;
  name: string;
  resource: string;
  startDate: Date; // Now includes hours, minutes, seconds
  endDate: Date; // Now includes hours, minutes, seconds
  duration?: number; // Duration in seconds (optional)
  percentComplete?: number;
  dependencies?: string[];
}

interface GanttChartProps {
  tasks: WorkflowTask[];
  title?: string;
  description?: string;
  height?: string | number;
  status?: WorkflowStatus;
}

const GanttChart: React.FC<GanttChartProps> = ({
  tasks,
  title = "Workflow Progress",
  description = "Visual representation of task progress and timeline",
  height = 400,
  status = "ongoing",
}) => {
  // Format data for Google Charts Gantt
  const formatData = () => {
    const columns = [
      { type: "string", label: "Task ID" },
      { type: "string", label: "Task Name" },
      { type: "string", label: "Resource" },
      { type: "date", label: "Start Date" },
      { type: "date", label: "End Date" },
      { type: "number", label: "Duration" },
      { type: "number", label: "Percent Complete" },
      { type: "string", label: "Dependencies" },
    ];

    const rows = tasks.map((task) => {
      // Calculate duration in seconds if not provided
      const calculatedDuration =
        task.duration ||
        Math.round((task.endDate.getTime() - task.startDate.getTime()) / 1000);

      return [
        task.id,
        task.name,
        task.resource,
        task.startDate,
        task.endDate,
        calculatedDuration,
        task.percentComplete || 0,
        task.dependencies ? task.dependencies.join(",") : null,
      ];
    });

    return [columns, ...rows];
  };

  const options = {
    gantt: {
      criticalPathEnabled: false,
      innerGridHorizLine: {
        stroke: "#ffe0b2",
        strokeWidth: 2,
      },
      innerGridTrack: { fill: "#fff3e0" },
      innerGridDarkTrack: { fill: "#ffcc80" },
      trackHeight: 30,
      barHeight: 20,
      defaultStartDate: new Date(2025, 4, 31, 9, 0, 0), // May 31, 2025, 9:00 AM
      arrow: {
        angle: 60,
        width: 2,
        color: "#5e97f6",
        radius: 0,
      },
    },
    hAxis: {
      format: "HH:mm:ss",
      minorGridlines: {
        count: 4,
      },
    },
  };

  // Status indicator style based on workflow status
  const getStatusIndicator = () => {
    const baseClasses = "inline-block w-3 h-3 rounded-full mr-2";

    switch (status) {
      case "completed":
        return (
          <span
            className={`${baseClasses} bg-green-500`}
            title="Completed"
          ></span>
        );
      case "ongoing":
        return (
          <span
            className={`${baseClasses} bg-blue-500 animate-pulse`}
            title="Ongoing"
          ></span>
        );
      case "pending":
        return (
          <span className={`${baseClasses} bg-gray-400`} title="Pending"></span>
        );
      default:
        return <span className={`${baseClasses} bg-gray-400`}></span>;
    }
  };

  return (
    <Card className="shadow-md">
      <CardHeader>
        <div className="flex items-center">
          {getStatusIndicator()}
          <CardTitle>{title}</CardTitle>
        </div>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        {tasks.length === 0 ? (
          <div className="flex justify-center items-center h-[300px] text-gray-500">
            No workflow tasks to display
          </div>
        ) : (
          <Chart
            chartType="Gantt"
            width="100%"
            height={height}
            data={formatData()}
            options={options}
          />
        )}
      </CardContent>
    </Card>
  );
};

export default GanttChart;
