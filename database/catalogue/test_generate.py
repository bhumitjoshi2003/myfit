import json,pathlib,sys,unittest
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parent))
import generate
class CatalogueTests(unittest.TestCase):
 def test_reference_anchors_and_recipe_math(self):
  foods,refs=generate.load();rows={r['name']:r for r in generate.build()}
  self.assertEqual(refs['USDA:09176']['values'],[60,0.82,14.98,0.38,1.6])
  self.assertAlmostEqual(refs['IFCT:A011']['values'][0],1480/4.184)
  self.assertAlmostEqual(refs['IFCT:A011']['values'][2],76.75+3.46)
  self.assertEqual(refs['HILIFE:soy']['values'],[345,52,33,.5,13])
  self.assertEqual(refs['USDA:05098']['description'],'CHICKEN,BROILERS OR FRYERS,THIGH,MEAT ONLY,CKD,RSTD')
  # Independent ingredient calculation for a dish, including all five nutrients.
  idli=rows['Idli, steamed'];rice=refs['USDA:20044']['values'];dal=refs['IFCT:B003']['values']
  for i,key in enumerate(['calories','protein','carbs','fat','fibre']):
   expected=generate.rounded((rice[i]+.35*dal[i])/4,0 if i==0 else 1)
   self.assertEqual(idli[key],expected)
  self.assertEqual(idli['servings'][-1]['grams'],40)
 def test_scope_units_and_generated_migration(self):
  rows=generate.build();self.assertEqual(len(rows),197)
  self.assertEqual(len({r['name'].strip().lower() for r in rows}),197)
  for row in rows:
   self.assertIn('Per 100 g edible portion',row['source'])
   self.assertEqual(row['servings'][0],{'name':'Grams','grams':1})
   self.assertEqual(row['servings'][1],{'name':'100 grams','grams':100})
   self.assertTrue(row['source'].startswith(generate.MARKER))
  self.assertEqual(generate.MIGRATION.read_text(),generate.render(rows))
  down=generate.MIGRATION.read_text().split('-- +goose Down')[1]
  self.assertNotIn('DELETE ',down);self.assertNotIn('DROP ',down)
if __name__=='__main__':unittest.main()
