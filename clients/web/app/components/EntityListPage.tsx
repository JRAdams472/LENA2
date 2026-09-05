"use client";

import { useQuery } from "@tanstack/react-query";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Alert from "@mui/material/Alert";
import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";

interface EntityListPageProps<T extends object> {
  title: string;
  queryKey: string[];
  queryFn: () => Promise<T[]>;
}

export default function EntityListPage<T extends object>({
  title,
  queryKey,
  queryFn,
}: EntityListPageProps<T>) {
  const { data, isLoading, error } = useQuery<T[]>({
    queryKey,
    queryFn,
  });

  const rows = data ?? [];
  const columns = rows.length > 0 ? Object.keys(rows[0]) : [];

  const getRowKey = (row: T): string =>
    Object.keys(row as Record<string, unknown>)
      .sort()
      .map((key) => `${key}:${JSON.stringify((row as Record<string, unknown>)[key])}`)
      .join("|");

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        {title}
      </Typography>
      <Paper sx={{ p: 2 }}>
        {isLoading && <CircularProgress />}
        {error && (
          <Alert severity="error">{(error as Error).message}</Alert>
        )}
        {!isLoading && !error && rows.length === 0 && (
          <Typography color="text.secondary">No data</Typography>
        )}
        {rows.length > 0 && (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  {columns.map((col) => (
                    <TableCell key={col}>{col}</TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={getRowKey(row)}>
                    {columns.map((col) => {
                      const value = (row as Record<string, unknown>)[col];
                      return (
                        <TableCell key={col}>
                          {value === null || value === undefined
                            ? ""
                            : typeof value === "object"
                            ? JSON.stringify(value)
                            : String(value)}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Paper>
    </Box>
  );
}
