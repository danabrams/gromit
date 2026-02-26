package methodology

import "testing"

func TestResolveAdapterMapsGoProfileToGoAdapter(t *testing.T) {
	t.Parallel()

	adapter, err := ResolveAdapter("go")
	if err != nil {
		t.Fatalf("ResolveAdapter(\"go\") returned error: %v", err)
	}

	// Verify it's a GoAdapter
	_, ok := adapter.(GoAdapter)
	if !ok {
		t.Errorf("ResolveAdapter(\"go\") returned type %T, want GoAdapter", adapter)
	}
}

func TestResolveAdapterMapsNodeProfileToPassthroughAdapter(t *testing.T) {
	t.Parallel()

	adapter, err := ResolveAdapter("node")
	if err != nil {
		t.Fatalf("ResolveAdapter(\"node\") returned error: %v", err)
	}

	// Verify it's a PassthroughAdapter
	_, ok := adapter.(PassthroughAdapter)
	if !ok {
		t.Errorf("ResolveAdapter(\"node\") returned type %T, want PassthroughAdapter", adapter)
	}
}

func TestResolveAdapterMapsPythonProfileToPassthroughAdapter(t *testing.T) {
	t.Parallel()

	adapter, err := ResolveAdapter("python")
	if err != nil {
		t.Fatalf("ResolveAdapter(\"python\") returned error: %v", err)
	}

	// Verify it's a PassthroughAdapter
	_, ok := adapter.(PassthroughAdapter)
	if !ok {
		t.Errorf("ResolveAdapter(\"python\") returned type %T, want PassthroughAdapter", adapter)
	}
}

func TestResolveAdapterMapsCustomProfileToPassthroughAdapter(t *testing.T) {
	t.Parallel()

	adapter, err := ResolveAdapter("custom")
	if err != nil {
		t.Fatalf("ResolveAdapter(\"custom\") returned error: %v", err)
	}

	// Verify it's a PassthroughAdapter
	_, ok := adapter.(PassthroughAdapter)
	if !ok {
		t.Errorf("ResolveAdapter(\"custom\") returned type %T, want PassthroughAdapter", adapter)
	}
}

func TestResolveAdapterCaseInsensitive(t *testing.T) {
	t.Parallel()

	adapter, err := ResolveAdapter("Go")
	if err != nil {
		t.Fatalf("ResolveAdapter(\"Go\") returned error: %v", err)
	}

	// Verify it's a GoAdapter
	_, ok := adapter.(GoAdapter)
	if !ok {
		t.Errorf("ResolveAdapter(\"Go\") returned type %T, want GoAdapter", adapter)
	}
}

func TestResolveAdapterErrorsOnUnknownProfile(t *testing.T) {
	t.Parallel()

	_, err := ResolveAdapter("unknown")
	if err == nil {
		t.Errorf("ResolveAdapter(\"unknown\") returned no error, want error")
	}
}
