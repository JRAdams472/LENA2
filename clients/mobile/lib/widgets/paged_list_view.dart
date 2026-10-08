import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

/// updateQuery for graphql_flutter's FetchMoreOptions: appends the new
/// page's items onto the connection under [field] and keeps the latest
/// pageInfo. Works for any `XxxPage { items pageInfo }` shape.
UpdateQuery appendPageItems(String field) {
  return (Map<String, dynamic>? previous, Map<String, dynamic>? more) {
    final prevPage = previous?[field] as Map<String, dynamic>?;
    final morePage = more?[field] as Map<String, dynamic>?;
    if (prevPage == null || morePage == null) {
      return more ?? previous;
    }
    final merged = Map<String, dynamic>.from(morePage);
    merged['items'] = [
      ...(prevPage['items'] as List? ?? []),
      ...(morePage['items'] as List? ?? []),
    ];
    return {...?previous, field: merged};
  };
}

/// The next page number to request given [loadedCount] rows of [pageSize]
/// pages — ceil handles a trailing partial page.
int nextPageFor(int loadedCount, int pageSize) =>
    (loadedCount + pageSize - 1) ~/ pageSize + 1;

/// A ListView.builder that calls [onLoadMore] when scrolled near the end
/// and shows a spinner row while more pages remain server-side.
///
/// [loadedCount] is the number of rows currently in memory; [totalCount]
/// is the server-reported pageInfo.totalCount for the same filter set.
class PagedListView extends StatefulWidget {
  const PagedListView({
    super.key,
    required this.loadedCount,
    required this.totalCount,
    required this.itemBuilder,
    required this.onLoadMore,
    this.padding = const EdgeInsets.all(16.0),
    this.physics,
    this.itemExtent,
  });

  final int loadedCount;
  final int totalCount;
  final IndexedWidgetBuilder itemBuilder;
  final Future<void> Function() onLoadMore;
  final EdgeInsetsGeometry padding;
  final ScrollPhysics? physics;
  final double? itemExtent;

  @override
  State<PagedListView> createState() => _PagedListViewState();
}

class _PagedListViewState extends State<PagedListView> {
  bool _loadingMore = false;

  bool get _hasMore => widget.loadedCount < widget.totalCount;

  Future<void> _loadMore() async {
    if (_loadingMore || !_hasMore) return;
    setState(() => _loadingMore = true);
    try {
      await widget.onLoadMore();
    } finally {
      if (mounted) setState(() => _loadingMore = false);
    }
  }

  bool _onScroll(ScrollNotification notification) {
    // extentAfter < 300 triggers a page ahead of the bottom so rows stream
    // in smoothly. ScrollMetricsNotification also fires on layout/content
    // changes, which covers a first page shorter than the viewport and a
    // merge that leaves the viewport underfilled — no build-time side
    // effects needed.
    if (notification.metrics.extentAfter < 300) {
      _loadMore();
    }
    return false;
  }

  @override
  Widget build(BuildContext context) {
    return NotificationListener<ScrollNotification>(
      onNotification: _onScroll,
      child: ListView.builder(
        padding: widget.padding,
        physics: widget.physics,
        itemExtent: widget.itemExtent,
        itemCount: widget.loadedCount + (_hasMore ? 1 : 0),
        itemBuilder: (context, index) {
          if (index >= widget.loadedCount) {
            return const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Center(
                child: SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            );
          }
          return widget.itemBuilder(context, index);
        },
      ),
    );
  }
}
