type WardrobeItem = {
  id: string;
  name: string;
  category: string;
  color: string;
};

export default async function Home() {
  let wardrobeItems: WardrobeItem[] = [];
  let error = "";

  try {
    const response = await fetch("http://localhost:8080/wardrobe", {
      cache: "no-store",
    });

    if (!response.ok) throw new Error();
    wardrobeItems = await response.json();
  } catch {
    error = "Could not load your wardrobe. Make sure the API is running.";
  }

  return (
    <main className="min-h-screen bg-stone-100 px-6 py-12 text-stone-900">
      <div className="mx-auto max-w-5xl">
        <header className="mb-10">
          <p className="text-sm uppercase tracking-widest text-stone-500">
            Digital Wardrobe
          </p>
          <h1 className="mt-2 text-4xl font-semibold">My Wardrobe</h1>
          <p className="mt-3 text-stone-600">
            Pieces you own and can style into outfits.
          </p>
        </header>

        {error && <p className="text-red-700">{error}</p>}

        <section className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {wardrobeItems.map((item) => (
            <article
              key={item.id}
              className="overflow-hidden rounded-2xl bg-white shadow-sm"
            >
              <div className="h-64 bg-stone-200" />

              <div className="p-5">
                <p className="text-sm text-stone-500">{item.category}</p>
                <h2 className="mt-1 text-xl font-medium">{item.name}</h2>
                <p className="mt-2 text-sm text-stone-600">{item.color}</p>
              </div>
            </article>
          ))}
        </section>
      </div>
    </main>
  );
}
