import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

const String aiAvailableQuery = r'''
  query AIAvailable {
    aiAvailable
  }
''';

const String askAssistantQuery = r'''
  query AskAssistant($question: String!) {
    askAssistant(question: $question) {
      answer
      toolCalls { name }
    }
  }
''';

class _Message {
  _Message.user(this.text)
      : isUser = true,
        tools = const [];
  _Message.assistant(this.text, this.tools) : isUser = false;
  final String text;
  final bool isUser;
  final List<String> tools;
}

/// Chat surface for the LENA assistant (askAssistant). Hidden affordances
/// stay consistent with the web: when the server reports aiAvailable=false
/// the screen explains the provider isn't configured instead of failing.
class AssistantScreen extends StatefulWidget {
  const AssistantScreen({super.key});

  @override
  State<AssistantScreen> createState() => _AssistantScreenState();
}

class _AssistantScreenState extends State<AssistantScreen> {
  final _messages = <_Message>[];
  final _input = TextEditingController();
  bool _available = false;
  bool _loading = true;
  bool _sending = false;
  String? _error;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // GraphQLProvider.of reads an inherited widget, so it must wait for
    // didChangeDependencies — calling it in initState throws.
    if (!_loading) return;
    _checkAvailable();
  }

  Future<void> _checkAvailable() async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
          document: gql(aiAvailableQuery), fetchPolicy: FetchPolicy.noCache),
    );
    if (!mounted) return;
    setState(() {
      _loading = false;
      _available = result.data?['aiAvailable'] == true;
    });
  }

  Future<void> _send(String question) async {
    final q = question.trim();
    if (q.isEmpty || _sending) return;
    _input.clear();
    setState(() {
      _messages.add(_Message.user(q));
      _sending = true;
      _error = null;
    });
    final client = GraphQLProvider.of(context).value;
    final result = await client.query(
      QueryOptions(
        document: gql(askAssistantQuery),
        variables: {'question': q},
        fetchPolicy: FetchPolicy.noCache,
      ),
    );
    if (!mounted) return;
    setState(() {
      _sending = false;
      if (result.hasException) {
        _error = result.exception.toString();
        return;
      }
      final answer = result.data?['askAssistant'];
      final tools = (answer?['toolCalls'] as List? ?? [])
          .map((t) => t['name'] as String)
          .toList();
      _messages
          .add(_Message.assistant(answer?['answer'] as String? ?? '', tools));
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (!_available) {
      return Scaffold(
        appBar: AppBar(title: const Text('Ask Dot')),
        body: const Padding(
          padding: EdgeInsets.all(16),
          child: Text(
            "Dot isn't configured on this server — no AI "
            'provider is set, so meal suggestions, event fixes, and pairing '
            'ideas are off too.',
          ),
        ),
      );
    }
    return Scaffold(
      appBar: AppBar(title: const Text('Ask Dot')),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: _messages.length,
              itemBuilder: (context, i) {
                final m = _messages[i];
                return Align(
                  alignment:
                      m.isUser ? Alignment.centerRight : Alignment.centerLeft,
                  child: Container(
                    margin: const EdgeInsets.symmetric(vertical: 4),
                    padding: const EdgeInsets.symmetric(
                        horizontal: 14, vertical: 10),
                    constraints: BoxConstraints(
                      maxWidth: MediaQuery.of(context).size.width * 0.8,
                    ),
                    decoration: BoxDecoration(
                      color: m.isUser
                          ? Theme.of(context).colorScheme.primary
                          : Theme.of(context)
                              .colorScheme
                              .surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(16),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          m.text,
                          style: TextStyle(
                            color: m.isUser ? Colors.white : null,
                          ),
                        ),
                        if (m.tools.isNotEmpty)
                          Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Text(
                              'looked up: ${m.tools.join(", ")}',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
          if (_sending)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 4),
              child: Text('Dot is thinking…'),
            ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ),
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _input,
                    decoration: const InputDecoration(
                      hintText: 'Ask Dot…',
                      border: OutlineInputBorder(),
                    ),
                    onSubmitted: _send,
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  icon: const Icon(Icons.send),
                  onPressed: _sending ? null : () => _send(_input.text),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
