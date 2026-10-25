import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../analytics/analytics.dart';

/// Form-field-style picker backed by a debounced, server-side search —
/// a dropdown of the first N rows cannot reach entities beyond page one.
/// Tapping opens [SearchPickerSheet]; selection reports the chosen row's
/// map (or null for the [noneLabel] row).
class SearchPickerField extends StatelessWidget {
  const SearchPickerField({
    super.key,
    required this.displayText,
    required this.onChanged,
    required this.sheetTitle,
    required this.document,
    required this.connectionField,
    required this.variablesFor,
    required this.itemLabel,
    required this.searchEntityType,
    this.label = '',
    this.noneLabel,
    this.hintText,
    this.isFlatList = false,
  });

  /// Text shown in the closed field (selection label, or empty).
  final String displayText;
  final ValueChanged<Map<String, dynamic>?> onChanged;

  /// Sheet configuration — see SearchPickerSheet.
  final String sheetTitle;
  final String document;
  final String connectionField;
  final Map<String, dynamic> Function(String searchTerm) variablesFor;
  final String Function(Map<String, dynamic> item) itemLabel;

  /// Analytics entity for recordSearch ('recipe', 'bottle', ...).
  final String searchEntityType;

  final String label;

  /// Label of the "clear selection" row; null hides it (required pick).
  final String? noneLabel;

  /// Placeholder shown in the closed field when nothing is selected.
  /// Defaults to [noneLabel].
  final String? hintText;

  /// When true, [connectionField] is a top-level `[T!]!` list (e.g.
  /// `categories`, `nutrientTypes`) rather than a `{items}` connection —
  /// the sheet fetches once and filters client-side on [itemLabel].
  final bool isFlatList;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: () => showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (_) => SearchPickerSheet(
          title: sheetTitle,
          document: document,
          connectionField: connectionField,
          variablesFor: variablesFor,
          itemLabel: itemLabel,
          searchEntityType: searchEntityType,
          noneLabel: noneLabel,
          isFlatList: isFlatList,
          onSelected: onChanged,
        ),
      ),
      child: InputDecorator(
        decoration: InputDecoration(labelText: label),
        child: Row(
          children: [
            Expanded(
              child: Text(
                displayText.isEmpty
                    ? (hintText ?? noneLabel ?? '')
                    : displayText,
                overflow: TextOverflow.ellipsis,
                style: displayText.isEmpty
                    ? TextStyle(color: Theme.of(context).hintColor)
                    : null,
              ),
            ),
            const Icon(Icons.search, size: 20),
          ],
        ),
      ),
    );
  }
}

/// Bottom sheet used by [SearchPickerField]: a debounced search field plus
/// the top server-ranked matches from [connectionField] in [document].
class SearchPickerSheet extends StatefulWidget {
  const SearchPickerSheet({
    super.key,
    required this.title,
    required this.document,
    required this.connectionField,
    required this.variablesFor,
    required this.itemLabel,
    required this.onSelected,
    required this.searchEntityType,
    this.noneLabel,
    this.isFlatList = false,
  });

  final String title;
  final String document;
  final String connectionField;
  final Map<String, dynamic> Function(String searchTerm) variablesFor;
  final String Function(Map<String, dynamic> item) itemLabel;
  final ValueChanged<Map<String, dynamic>?> onSelected;
  final String searchEntityType;
  final String? noneLabel;

  /// See [SearchPickerField.isFlatList].
  final bool isFlatList;

  @override
  State<SearchPickerSheet> createState() => _SearchPickerSheetState();
}

class _SearchPickerSheetState extends State<SearchPickerSheet> {
  final _searchCtrl = TextEditingController();
  final _debouncer = Debouncer();
  List<Map<String, dynamic>> _results = [];
  // Flat-list mode keeps the whole fetched vocabulary here and filters
  // into _results — no per-keystroke round-trips.
  List<Map<String, dynamic>> _allItems = [];
  bool _loading = true;
  bool _loadedOnce = false;
  String _search = '';
  Object? _error;
  // Monotonic request token — a slower response arriving after a newer
  // search was issued must not overwrite fresher results.
  int _requestSeq = 0;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // GraphQLProvider.of is an inherited lookup — it cannot run in
    // initState, so the first page loads here.
    if (!_loadedOnce) {
      _loadedOnce = true;
      _load();
    }
  }

  @override
  void dispose() {
    _debouncer.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  List<Map<String, dynamic>> _filtered(List<Map<String, dynamic>> all) {
    if (_search.isEmpty) return all;
    final needle = _search.toLowerCase();
    return all
        .where((i) => widget.itemLabel(i).toLowerCase().contains(needle))
        .toList();
  }

  Future<void> _load() async {
    final seq = ++_requestSeq;
    final client = GraphQLProvider.of(context).value;
    try {
      final result = await client.query(
        QueryOptions(
          document: gql(widget.document),
          fetchPolicy: FetchPolicy.noCache,
          variables: widget.variablesFor(_search),
        ),
      );
      // A newer request (or dispose) issued while this one was in flight —
      // its response owns the results now.
      if (!mounted || seq != _requestSeq) return;
      setState(() {
        _loading = false;
        _error = result.hasException ? result.exception : null;
        if (widget.isFlatList) {
          _allItems = (result.data?[widget.connectionField] as List? ?? [])
              .cast<Map<String, dynamic>>();
          _results = _filtered(_allItems);
        } else {
          _results =
              (result.data?[widget.connectionField]?['items'] as List? ?? [])
                  .cast<Map<String, dynamic>>();
        }
      });
    } catch (e) {
      if (!mounted || seq != _requestSeq) return;
      setState(() {
        _loading = false;
        _error = e;
      });
    }
  }

  void _onSearchChanged(String value) {
    _debouncer.run(() {
      final term = value.trim();
      if (term == _search) return;
      // Flat lists filter the already-fetched vocabulary — no refetch.
      if (widget.isFlatList) {
        setState(() {
          _search = term;
          _results = _filtered(_allItems);
        });
        return;
      }
      setState(() {
        _search = term;
        _loading = true;
        _error = null;
      });
      if (term.isNotEmpty) {
        recordSearch(
            GraphQLProvider.of(context).value, widget.searchEntityType, term);
      }
      _load();
    });
  }

  void _retry() {
    setState(() {
      _loading = true;
      _error = null;
    });
    _load();
  }

  void _pick(Map<String, dynamic>? item) {
    widget.onSelected(item);
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: SizedBox(
        height: MediaQuery.of(context).size.height * 0.6,
        child: Column(
          children: [
            TextField(
              controller: _searchCtrl,
              autofocus: true,
              decoration: InputDecoration(
                labelText: widget.title,
                prefixIcon: const Icon(Icons.search),
              ),
              onChanged: _onSearchChanged,
            ),
            const SizedBox(height: 8),
            Expanded(
              // The noneLabel row stays reachable through loading and
              // error states — clearing a pick must never depend on the
              // server answering.
              child: ListView(
                children: [
                  if (widget.noneLabel != null)
                    ListTile(
                      leading: const Icon(Icons.block),
                      title: Text(widget.noneLabel!),
                      onTap: () => _pick(null),
                    ),
                  if (_loading)
                    const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: CircularProgressIndicator()),
                    )
                  else if (_error != null)
                    Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        children: [
                          Text('Could not load: $_error',
                              textAlign: TextAlign.center),
                          const SizedBox(height: 8),
                          TextButton.icon(
                            onPressed: _retry,
                            icon: const Icon(Icons.refresh),
                            label: const Text('Retry'),
                          ),
                        ],
                      ),
                    )
                  else ...[
                    for (final r in _results)
                      ListTile(
                        title: Text(widget.itemLabel(r)),
                        onTap: () => _pick(r),
                      ),
                    if (_results.isEmpty)
                      const Padding(
                        padding: EdgeInsets.all(24),
                        child: Center(child: Text('No matches.')),
                      ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
