﻿using System;
using System.Net;
using System.IO;

using NLog;
using Newtonsoft.Json.Linq;

namespace ZitiDesktopEdge.Utility {
    public static class GithubAPI {
		private static readonly Logger Logger = LogManager.GetCurrentClassLogger();

		public const string ProdUrl = "https://get.openziti.io/zdew/stable.json";
		
		public static JObject GetJson(string url) {
			HttpWebRequest httpWebRequest = WebRequest.CreateHttp(url);
			httpWebRequest.Method = "GET";
			httpWebRequest.ContentType = "application/json";
			httpWebRequest.UserAgent = "OpenZiti UpdateService";
			HttpWebResponse httpResponse = (HttpWebResponse)httpWebRequest.GetResponse();
			StreamReader streamReader = new StreamReader(httpResponse.GetResponseStream());
			string currentResponse = streamReader.ReadToEnd();
			Logger.Trace("response received: {0}", currentResponse);
			return JObject.Parse(currentResponse);
		}
		public static JArray GetJsonArray(string url) {
			HttpWebRequest httpWebRequest = WebRequest.CreateHttp(url);
			httpWebRequest.Method = "GET";
			httpWebRequest.ContentType = "application/json";
			httpWebRequest.UserAgent = "OpenZiti UpdateService";
			HttpWebResponse httpResponse = (HttpWebResponse)httpWebRequest.GetResponse();
			StreamReader streamReader = new StreamReader(httpResponse.GetResponseStream());
			string currentResponse = streamReader.ReadToEnd();
			Logger.Trace("response received for url: {0}", url);
			return JArray.Parse(currentResponse);
		}

		public static Version GetVersion(JObject json) {
			string releaseVersion = json.Property("tag_name").Value.ToString();
			string releaseName = json.Property("name").Value.ToString();
			return VersionUtil.NormalizeVersion(new Version(releaseVersion));
		}
	}
}
