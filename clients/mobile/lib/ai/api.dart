/// GraphQL plumbing for the assistant's client-facing endpoints added in
/// the local-AI server phase: tool catalog, scoped read-only tool
/// dispatch, and the server-owned system prompt.
library;

import 'package:graphql_flutter/graphql_flutter.dart';

import 'engine.dart';

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

const String assistantToolsQuery = r'''
  query AssistantTools {
    assistantTools { name description parametersJson }
  }
''';

const String assistantPromptQuery = r'''
  query AssistantPrompt($name: String!) {
    assistantPrompt(name: $name)
  }
''';

const String callAssistantToolQuery = r'''
  query CallAssistantTool($name: String!, $arguments: String!) {
    callAssistantTool(name: $name, arguments: $arguments)
  }
''';

/// Thin wrapper so the controller and agent are testable without a live
/// GraphQL client — tests substitute a fake with the same surface.
abstract class AssistantApi {
  Future<bool> aiAvailable();
  Future<({String answer, List<String> tools})> askAssistant(String question);
  Future<List<AgentToolSpec>> assistantTools();
  Future<String> assistantPrompt(String name);
  Future<String> callAssistantTool(String name, String argumentsJson);
}

class GraphQLAssistantApi implements AssistantApi {
  GraphQLAssistantApi(this._client);
  final GraphQLClient _client;

  Future<QueryResult> _run(String document,
      [Map<String, dynamic> variables = const {}]) {
    return _client.query(
      QueryOptions(
        document: gql(document),
        variables: variables,
        fetchPolicy: FetchPolicy.noCache,
      ),
    );
  }

  @override
  Future<bool> aiAvailable() async {
    final result = await _run(aiAvailableQuery);
    if (result.hasException) return false;
    return result.data?['aiAvailable'] == true;
  }

  @override
  Future<({String answer, List<String> tools})> askAssistant(
      String question) async {
    final result = await _run(askAssistantQuery, {'question': question});
    if (result.hasException) {
      throw result.exception ?? Exception('askAssistant failed');
    }
    final answer = result.data?['askAssistant'];
    final tools = ((answer?['toolCalls'] as List?) ?? [])
        .map((t) => t['name'] as String)
        .toList();
    return (answer: answer?['answer'] as String? ?? '', tools: tools);
  }

  @override
  Future<List<AgentToolSpec>> assistantTools() async {
    final result = await _run(assistantToolsQuery);
    if (result.hasException) {
      throw result.exception ?? Exception('assistantTools failed');
    }
    return ((result.data?['assistantTools'] as List?) ?? []).map((t) {
      return AgentToolSpec(
        name: t['name'] as String,
        description: t['description'] as String? ?? '',
        parametersJson: t['parametersJson'] as String? ?? '{}',
      );
    }).toList();
  }

  @override
  Future<String> assistantPrompt(String name) async {
    final result = await _run(assistantPromptQuery, {'name': name});
    if (result.hasException) {
      throw result.exception ?? Exception('assistantPrompt failed');
    }
    return result.data?['assistantPrompt'] as String? ?? '';
  }

  @override
  Future<String> callAssistantTool(String name, String argumentsJson) async {
    final result = await _run(
      callAssistantToolQuery,
      {'name': name, 'arguments': argumentsJson},
    );
    if (result.hasException) {
      throw result.exception ?? Exception('callAssistantTool failed');
    }
    return result.data?['callAssistantTool'] as String? ?? '';
  }
}
