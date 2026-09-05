import "@testing-library/jest-dom";

process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:5059/graphql";
process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID = "test-google-client-id";

global.fetch = jest.fn();
