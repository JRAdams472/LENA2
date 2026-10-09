import 'package:flutter_test/flutter_test.dart';
import 'package:gql/ast.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/ai/api.dart';

String? _opName(Request request) {
  for (final def in request.operation.document.definitions) {
    if (def is OperationDefinitionNode) return def.name?.value;
  }
  return null;
}

class _CaptureLink extends Link {
  final List<Request> requests = [];
  final Map<String, Map<String, dynamic>> responses;
  final Map<String, List<GraphQLError>> errors;

  _CaptureLink(this.responses, {this.errors = const {}});

  @override
  Stream<Response> request(Request request, [NextLink? forward]) async* {
    requests.add(request);
    yield Response(
      data: responses[_opName(request)] ?? <String, dynamic>{},
      errors: errors[_opName(request)],
      response: const <String, dynamic>{},
    );
  }

  List<Request> byName(String name) =>
      requests.where((r) => _opName(r) == name).toList();
}

GraphQLAssistantApi _api(_CaptureLink link) => GraphQLAssistantApi(
      GraphQLClient(cache: GraphQLCache(), link: link),
    );

void main() {
  test('aiAvailable returns the server flag', () async {
    final api = _api(_CaptureLink({
      'AIAvailable': {'aiAvailable': true}
    }));
    expect(await api.aiAvailable(), isTrue);
  });

  test('aiAvailable is false when the flag is false or missing', () async {
    expect(
      await _api(_CaptureLink({
        'AIAvailable': {'aiAvailable': false}
      })).aiAvailable(),
      isFalse,
    );
    // Missing key entirely — the null-safe lookup still reports false.
    expect(await _api(_CaptureLink({})).aiAvailable(), isFalse);
  });

  test('aiAvailable swallows query errors as unavailable', () async {
    final api = _api(
      _CaptureLink({}, errors: {
        'AIAvailable': [const GraphQLError(message: 'boom')],
      }),
    );
    expect(await api.aiAvailable(), isFalse);
  });

  test('askAssistant returns the answer and tool names', () async {
    final link = _CaptureLink({
      'AskAssistant': {
        'askAssistant': {
          'answer': 'Use the rice tonight.',
          'toolCalls': [
            {'name': 'get_pantry_inventory'},
            {'name': 'list_recipes'},
          ],
        },
      },
    });
    final api = _api(link);

    final out = await api.askAssistant('what is for dinner');

    expect(out.answer, 'Use the rice tonight.');
    expect(out.tools, ['get_pantry_inventory', 'list_recipes']);
    final req = link.byName('AskAssistant').single;
    expect(req.variables['question'], 'what is for dinner');
  });

  test('askAssistant tolerates a null answer object', () async {
    final api = _api(_CaptureLink({'AskAssistant': {}}));
    final out = await api.askAssistant('q');
    expect(out.answer, '');
    expect(out.tools, isEmpty);
  });

  test('askAssistant throws the transport exception', () async {
    final api = _api(
      _CaptureLink({}, errors: {
        'AskAssistant': [const GraphQLError(message: 'no provider')],
      }),
    );
    expect(() => api.askAssistant('q'), throwsA(isA<OperationException>()));
  });

  test('assistantTools maps the catalog rows to specs', () async {
    final api = _api(_CaptureLink({
      'AssistantTools': {
        'assistantTools': [
          {
            'name': 'get_pantry_inventory',
            'description': 'Read pantry',
            'parametersJson': '{"type":"object"}',
          },
          {'name': 'sparse'},
        ],
      },
    }));

    final tools = await api.assistantTools();

    expect(tools, hasLength(2));
    expect(tools.first.name, 'get_pantry_inventory');
    expect(tools.first.description, 'Read pantry');
    expect(tools.first.parametersJson, '{"type":"object"}');
    // Sparse rows fall back to empty description / empty schema.
    expect(tools.last.description, '');
    expect(tools.last.parametersJson, '{}');
  });

  test('assistantTools throws on error and returns empty on no rows', () async {
    final bad = _api(
      _CaptureLink({}, errors: {
        'AssistantTools': [const GraphQLError(message: 'down')],
      }),
    );
    expect(() => bad.assistantTools(), throwsA(isA<OperationException>()));
    expect(await _api(_CaptureLink({})).assistantTools(), isEmpty);
  });

  test('assistantPrompt returns the prompt body and passes the name', () async {
    final link = _CaptureLink({
      'AssistantPrompt': {'assistantPrompt': 'You are Dot.'},
    });
    final api = _api(link);

    expect(await api.assistantPrompt('ask'), 'You are Dot.');
    expect(link.byName('AssistantPrompt').single.variables['name'], 'ask');
  });

  test('assistantPrompt throws on error and defaults to empty', () async {
    final bad = _api(
      _CaptureLink({}, errors: {
        'AssistantPrompt': [const GraphQLError(message: 'missing')],
      }),
    );
    expect(
        () => bad.assistantPrompt('ask'), throwsA(isA<OperationException>()));
    expect(await _api(_CaptureLink({})).assistantPrompt('ask'), '');
  });

  test('callAssistantTool passes name and serialized arguments', () async {
    final link = _CaptureLink({
      'CallAssistantTool': {'callAssistantTool': '{"count":3}'},
    });
    final api = _api(link);

    expect(await api.callAssistantTool('get_pantry', '{"a":1}'), '{"count":3}');
    final req = link.byName('CallAssistantTool').single;
    expect(req.variables['name'], 'get_pantry');
    expect(req.variables['arguments'], '{"a":1}');
  });

  test('callAssistantTool throws on error and defaults to empty', () async {
    final bad = _api(
      _CaptureLink({}, errors: {
        'CallAssistantTool': [const GraphQLError(message: 'denied')],
      }),
    );
    expect(() => bad.callAssistantTool('x', '{}'),
        throwsA(isA<OperationException>()));
    expect(await _api(_CaptureLink({})).callAssistantTool('x', '{}'), '');
  });
}
