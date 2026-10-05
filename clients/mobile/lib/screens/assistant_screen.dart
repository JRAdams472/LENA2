import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';

import '../ai/api.dart';
import '../ai/controller.dart';
import '../ai/gemma_binding.dart';
import '../ai/gemma_engine.dart';
import '../ai/model_manager.dart';

class _Message {
  _Message.user(this.text)
      : isUser = true,
        tools = const [],
        engineLabel = null;
  _Message.assistant(this.text, this.tools, {this.engineLabel})
      : isUser = false;
  final String text;
  final bool isUser;
  final List<String> tools;
  final String? engineLabel;
}

/// Chat surface for the LENA assistant. Ask Dot prefers the on-device
/// Gemma model once the user has opted in and downloaded it; every tool
/// call still executes server-side under the caller's household scope, and
/// the server answers directly whenever no local model is ready.
class AssistantScreen extends StatefulWidget {
  const AssistantScreen({super.key, this.controller});

  /// Injectable for tests; when null the screen wires the real
  /// GraphQL-backed controller in didChangeDependencies.
  final AssistantController? controller;

  @override
  State<AssistantScreen> createState() => _AssistantScreenState();
}

class _AssistantScreenState extends State<AssistantScreen> {
  final _messages = <_Message>[];
  final _input = TextEditingController();
  late AssistantController _controller;
  bool _ownsController = false;
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _controller = widget.controller ?? _buildController();
    _ownsController = widget.controller == null;
    _controller.addListener(_onControllerChanged);
    _controller.initialize();
  }

  AssistantController _buildController() {
    final binding = FlutterGemmaBinding();
    return AssistantController(
      api: GraphQLAssistantApi(GraphQLProvider.of(context).value),
      modelManager: LocalModelManager(binding),
      engineFactory: () => GemmaEngine(binding),
    );
  }

  void _onControllerChanged() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _controller.removeListener(_onControllerChanged);
    if (_ownsController) _controller.dispose();
    _input.dispose();
    super.dispose();
  }

  Future<void> _send(String question) async {
    final q = question.trim();
    if (q.isEmpty || _controller.sending) return;
    _input.clear();
    setState(() => _messages.add(_Message.user(q)));
    try {
      final result = await _controller.ask(q);
      if (!mounted) return;
      setState(() {
        _messages.add(
          _Message.assistant(
            result.answer,
            result.tools,
            engineLabel: result.engineLabel,
          ),
        );
      });
    } catch (_) {
      // The controller surfaced the error text; nothing else to do.
    }
  }

  // Same quick prompts the web chat offers — tap one to start a
  // conversation instead of staring at an empty list.
  Widget _emptyState() {
    const prompts = [
      "What's expiring in my pantry soon?",
      'What should I cook this week?',
      "What's in my wine cellar?",
    ];
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        const SizedBox(height: 32),
        Text(
          'Ask Dot anything about your kitchen.',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 16),
        Wrap(
          alignment: WrapAlignment.center,
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final p in prompts)
              ActionChip(
                label: Text(p),
                onPressed: _controller.sending ? null : () => _send(p),
              ),
          ],
        ),
      ],
    );
  }

  Widget _buildLocalCard() {
    final c = _controller;
    if (c.status == LocalStatus.downloading) {
      final p = c.downloadProgress;
      return Card(
        margin: const EdgeInsets.all(12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('Downloading the on-device model…'),
              const SizedBox(height: 8),
              LinearProgressIndicator(
                value: p == null || p <= 0 ? null : p / 100,
              ),
              const SizedBox(height: 4),
              Text(
                p == null ? '' : '$p%',
                style: Theme.of(context).textTheme.bodySmall,
              ),
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: c.cancelDownload,
                  child: const Text('Cancel'),
                ),
              ),
            ],
          ),
        ),
      );
    }
    if (c.status == LocalStatus.optIn) {
      return Card(
        margin: const EdgeInsets.all(12),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Run Dot on this device',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 4),
              const Text(
                'Download a small model (~550 MB) once and Dot can answer '
                'on this device — faster replies, and your questions never '
                'leave the phone. Dot still reads household data through '
                'the server.',
              ),
              if (c.downloadError != null) ...[
                const SizedBox(height: 6),
                Text(
                  'Last download attempt failed.',
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.error,
                  ),
                ),
              ],
              Align(
                alignment: Alignment.centerRight,
                child: FilledButton.tonal(
                  onPressed: c.enableLocal,
                  child: const Text('Download model'),
                ),
              ),
            ],
          ),
        ),
      );
    }
    return const SizedBox.shrink();
  }

  @override
  Widget build(BuildContext context) {
    final c = _controller;

    if (c.status == LocalStatus.checking) {
      return const Scaffold(body: SkeletonList());
    }
    if (!c.available) {
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
      appBar: AppBar(
        title: const Text('Ask Dot'),
        actions: [
          if (c.modelInstalled || c.localActive)
            Padding(
              padding: const EdgeInsets.only(right: 4),
              child: Center(
                child: Text(
                  c.engineLabel,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
            ),
          if (c.modelInstalled)
            PopupMenuButton<String>(
              iconSize: 20,
              onSelected: (v) {
                if (v == 'server') c.setServerOnly(true);
                if (v == 'local') c.setServerOnly(false);
                if (v == 'delete') c.deleteModel();
              },
              itemBuilder: (context) => [
                if (c.mode == AssistantMode.server)
                  const PopupMenuItem(
                      value: 'local', child: Text('Use on-device model'))
                else
                  const PopupMenuItem(
                      value: 'server', child: Text('Use server answers')),
                const PopupMenuItem(
                    value: 'delete', child: Text('Delete on-device model')),
              ],
            ),
        ],
      ),
      body: Column(
        children: [
          _buildLocalCard(),
          Expanded(
            child: _messages.isEmpty
                ? _emptyState()
                : ListView.builder(
                    padding: const EdgeInsets.all(12),
                    itemCount: _messages.length,
                    itemBuilder: (context, i) {
                      final m = _messages[i];
                      return Align(
                        alignment: m.isUser
                            ? Alignment.centerRight
                            : Alignment.centerLeft,
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
                                  color: m.isUser
                                      ? Theme.of(context).colorScheme.onPrimary
                                      : null,
                                ),
                              ),
                              if (m.tools.isNotEmpty)
                                Padding(
                                  padding: const EdgeInsets.only(top: 4),
                                  child: Text(
                                    'looked up: ${m.tools.join(", ")}',
                                    style: Theme.of(context)
                                        .textTheme
                                        .bodySmall
                                        ?.copyWith(
                                          color: m.isUser
                                              ? Theme.of(context)
                                                  .colorScheme
                                                  .onPrimary
                                                  .withValues(alpha: 0.8)
                                              : null,
                                        ),
                                  ),
                                ),
                              if (m.engineLabel != null)
                                Padding(
                                  padding: const EdgeInsets.only(top: 4),
                                  child: Text(
                                    m.engineLabel!,
                                    style: Theme.of(context)
                                        .textTheme
                                        .labelSmall
                                        ?.copyWith(
                                          letterSpacing: 0,
                                          color: m.isUser
                                              ? Theme.of(context)
                                                  .colorScheme
                                                  .onPrimary
                                                  .withValues(alpha: 0.7)
                                              : null,
                                        ),
                                  ),
                                ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
          ),
          if (c.sending)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 4),
              child: Text('Dot is thinking…'),
            ),
          if (c.error != null)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Text(c.error!,
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
                  onPressed: c.sending ? null : () => _send(_input.text),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
